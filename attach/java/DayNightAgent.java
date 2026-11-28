package lab.daynight.attach;

import java.io.*;
import java.lang.instrument.*;
import java.lang.management.ManagementFactory;
import java.lang.reflect.*;
import java.net.*;
import java.nio.file.*;
import java.security.*;
import java.util.*;
import java.util.jar.*;

/** Java 8 agent: update pinned menu instances without redefining JVM classes. */
public final class DayNightAgent {
    private static String helperPrefix;
    private static Instrumentation instrumentation;
    private static ClassLoader viewLoader;
    private static Class<?> runtimeViews;
    private static final PatchSession session = new PatchSession();
    private static int lastRefreshCount;
    private static JarFile bootstrapHelpers;

    public static synchronized void agentmain(String options, Instrumentation inst) throws Exception {
        try (AgentRequest request = AgentRequest.open(options)) {
            instrumentation = inst;
            Throwable failure = null;
            try {
                if ("install".equals(request.action)) install();
                else if ("remove".equals(request.action)) remove();
                else if (session.installed()) onUI(() -> runtimeViews.getMethod("checkMonitor").invoke(null));
            } catch (Throwable error) { failure = error; }
            try { request.write(resultJson(failure == null, request.action, failure)); }
            catch (Exception reportError) {
                if (failure == null) throw reportError;
                failure.addSuppressed(reportError);
            }
            if (failure instanceof Error) throw (Error)failure;
            if (failure instanceof Exception) throw (Exception)failure;
            if (failure != null) throw new RuntimeException(failure);
        }
    }

    private static void install() throws Exception {
        session.requireInstallable();
        if (session.installed()) return;
        final Properties pins = new Properties();
        try (InputStream in = resource("pins.properties")) { pins.load(in); }
        List<String> names = new ArrayList<String>(pins.stringPropertyNames());
        Collections.sort(names);
        if (names.isEmpty()) throw new SecurityException("Empty patch target set");
        Class<?> root = null;
        for (Class<?> type : instrumentation.getAllLoadedClasses()) {
            if (names.contains(type.getName())) {
                if (root != null && root.getClassLoader() != type.getClassLoader())
                    throw new SecurityException("Multiple target skin class loaders");
                root = type;
            }
        }
        if (root == null) throw new IllegalStateException("Open the original control center before attaching");
        Properties firmware = new Properties();
        try (InputStream in = resource("firmware.properties")) { firmware.load(in); }
        if (names.size() != Integer.parseInt(firmware.getProperty("target.count")))
            throw new SecurityException("Target count differs from selected version");
        helperPrefix = firmware.getProperty("helper.prefix");
        viewLoader = root.getClassLoader();
        URL viewSource = classLocation(root);
        verifyFile(viewSource, firmware.getProperty("view.sha256"));
        Class<?> hmi = Class.forName("de.vw.mib.MIBMain", false, viewLoader);
        verifyFile(classLocation(hmi), firmware.getProperty("hmi.sha256"));
        for (String name : names) {
            Class<?> type = Class.forName(name, false, viewLoader);
            byte[] disk = read(type.getResourceAsStream("/" + name.replace('.', '/') + ".class"));
            if (!sha(disk).equals(pins.getProperty(name))) throw new SecurityException("Unknown target class: " + name);
        }
        loadHelpers();
        onUI(() -> session.install(changes()));
        System.out.println("DAYNIGHT_ATTACH installed liveNodes=" + lastRefreshCount);
    }

    private static PatchSession.Changes changes() {
        return new PatchSession.Changes() {
            public void apply() throws Exception { lastRefreshCount = refresh(true); }
            public void restore() throws Exception { lastRefreshCount = refresh(false); }
        };
    }

    private static void remove() throws Exception {
        if (session.original()) return;
        onUI(() -> session.remove(changes()));
        System.out.println("DAYNIGHT_ATTACH removed; original menu state restored");
    }

    private static void loadHelpers() throws Exception {
        Path self = Paths.get(DayNightAgent.class.getProtectionDomain().getCodeSource().getLocation().toURI());
        Path helper = self.resolveSibling("daynight-helpers.jar");
        Properties meta = new Properties();
        try (InputStream in = resource("bundle.properties")) { meta.load(in); }
        if (!sha(Files.readAllBytes(helper)).equals(meta.getProperty("helpers.sha256")))
            throw new SecurityException("Helper JAR hash mismatch");
        try { Class.forName(helperPrefix + "DayNightSelection", false, viewLoader);
            if (runtimeViews == null) throw new SecurityException("A different day/night helper is already loaded");
        } catch (ClassNotFoundException missing) {
            if (viewLoader == null) {
                bootstrapHelpers = new JarFile(helper.toFile());
                instrumentation.appendToBootstrapClassLoaderSearch(bootstrapHelpers);
            } else {
                Method define = ClassLoader.class.getDeclaredMethod("defineClass", String.class, byte[].class, int.class, int.class);
                define.setAccessible(true);
                try (JarFile jar = new JarFile(helper.toFile())) {
                    List<String> entries = Collections.list(jar.entries()).stream()
                        .filter(e -> e.getName().endsWith(".class")).map(JarEntry::getName)
                        .sorted().collect(java.util.stream.Collectors.toList());
                    for (String name : entries) {
                        byte[] bytes = read(jar.getInputStream(jar.getJarEntry(name)));
                        define.invoke(viewLoader, name.replace('/', '.').replace(".class", ""), bytes, 0, bytes.length);
                    }
                }
            }
        }
        runtimeViews = Class.forName(helperPrefix + "RuntimeViews", true, viewLoader);
    }

    private static int refresh(boolean apply) throws Exception {
        try { return ((Integer) runtimeViews.getMethod("refresh", boolean.class).invoke(null, apply)).intValue(); }
        catch (InvocationTargetException error) { throw new Exception(error.getCause()); }
    }

    private static void onUI(UiExecutor.Work work) throws Exception {
        Class<?> services = Class.forName("de.vw.mib.widgets.internal.ServiceManager", false, viewLoader);
        final Object dispatcher = services.getField("eventDispatcher").get(null);
        if (dispatcher == null) throw new IllegalStateException("HMI UI dispatcher unavailable");
        final Class<?> invoker = Class.forName("de.vw.mib.timer.Invoker", false, viewLoader);
        UiExecutor.execute(new UiExecutor.Dispatcher() {
            public void invoke(Runnable task) throws Exception {
                invoker.getMethod("invoke", Runnable.class).invoke(dispatcher, task);
            }
        }, work, 15000);
    }

    private static void verifyFile(URL url, String expected) throws Exception {
        if (url == null || !"file".equals(url.getProtocol())) throw new SecurityException("Expected local firmware JAR");
        if (!sha(Files.readAllBytes(Paths.get(url.toURI()))).equals(expected)) throw new SecurityException("Unsupported firmware JAR: " + url);
    }
    private static URL classLocation(Class<?> type) throws Exception {
        CodeSource source = type.getProtectionDomain().getCodeSource();
        if (source != null) return source.getLocation();
        URL resource = type.getResource("/" + type.getName().replace('.', '/') + ".class");
        if (resource == null || !"jar".equals(resource.getProtocol()))
            throw new SecurityException("Cannot verify original class source: " + type.getName());
        return ((JarURLConnection)resource.openConnection()).getJarFileURL();
    }
    private static InputStream resource(String name) throws IOException {
        try {
            Path self = Paths.get(DayNightAgent.class.getProtectionDomain().getCodeSource().getLocation().toURI());
            try (JarFile jar = new JarFile(self.toFile())) {
                JarEntry entry = jar.getJarEntry(name);
                if (entry == null) throw new FileNotFoundException(name);
                return new ByteArrayInputStream(read(jar.getInputStream(entry)));
            }
        } catch (URISyntaxException error) { throw new IOException(error); }
    }
    private static byte[] read(InputStream input) throws IOException {
        if (input == null) throw new FileNotFoundException("Missing class bytes");
        try (InputStream in = input; ByteArrayOutputStream out = new ByteArrayOutputStream()) {
            byte[] block = new byte[16384]; int count;
            while ((count = in.read(block)) != -1) out.write(block, 0, count);
            return out.toByteArray();
        }
    }
    private static String sha(byte[] bytes) throws Exception {
        StringBuilder result = new StringBuilder();
        for (byte b : MessageDigest.getInstance("SHA-256").digest(bytes)) result.append(String.format("%02x", b & 255));
        return result.toString();
    }
    private static String resultJson(boolean ok, String action, Throwable failure) {
        return "{\"ok\":" + ok + ",\"action\":" + AgentRequest.quote(action)
            + ",\"installed\":" + session.installed() + ",\"state\":" + AgentRequest.quote(session.state().name())
            + ",\"runtime\":" + AgentRequest.quote(ManagementFactory.getRuntimeMXBean().getName())
            + ",\"start_time_ms\":" + ManagementFactory.getRuntimeMXBean().getStartTime()
            + ",\"redefine_supported\":" + instrumentation.isRedefineClassesSupported()
            + ",\"retransform_supported\":" + instrumentation.isRetransformClassesSupported()
            + ",\"view_loader\":" + AgentRequest.quote(viewLoader == null ? "bootstrap" : viewLoader.getClass().getName())
            + ",\"live_nodes\":" + lastRefreshCount + ",\"error\":"
            + (failure == null ? "null" : AgentRequest.quote(failure.toString())) + "}\n";
    }
}
