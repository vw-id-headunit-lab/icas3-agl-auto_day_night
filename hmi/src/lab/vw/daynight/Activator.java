package lab.vw.daynight;

import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.nio.file.attribute.PosixFilePermission;
import java.util.Properties;
import java.util.Set;
import org.osgi.framework.BundleActivator;
import org.osgi.framework.BundleContext;

/** Optional late-load HMI component. Never changes a mode during startup. */
public final class Activator implements BundleActivator {
    private BridgeServer server;
    public void start(BundleContext context) {
        try {
            Path path=Paths.get("/data/vw-daynight/bridge.properties");
            if (!Files.exists(path,LinkOption.NOFOLLOW_LINKS)) return;
            checkPrivate(path);
            Properties p=new Properties();try(InputStream in=Files.newInputStream(path)){p.load(in);}
            if (!Boolean.parseBoolean(p.getProperty("enabled","false"))) return;
            int port=Integer.parseInt(p.getProperty("port","47851"));if(port<1024||port>65535) throw new IllegalArgumentException("port");
            Path tokenPath=Paths.get(p.getProperty("token_file","/data/vw-daynight/bridge.token"));
            checkPrivate(tokenPath);
            String token=new String(Files.readAllBytes(tokenPath),StandardCharsets.US_ASCII).trim();
            if (!token.matches("[0-9a-fA-F]{64}")) throw new IllegalArgumentException("token");
            server=new BridgeServer(new QueuedBackend(new FirmwareRuntime(),6000),token,
                Boolean.parseBoolean(p.getProperty("allow_write","false")),port);
            server.start();System.out.println("[vw-daynight] bridge started on loopback; allow_write="+p.getProperty("allow_write","false"));
        } catch (Exception|LinkageError e) {
            System.err.println("[vw-daynight] optional bridge unavailable: "+e.getClass().getSimpleName());
        }
    }
    private static void checkPrivate(Path path) throws Exception {
        if (!Files.isRegularFile(path,LinkOption.NOFOLLOW_LINKS)) throw new IllegalArgumentException("regular file required");
        Set<PosixFilePermission> perms=Files.getPosixFilePermissions(path,LinkOption.NOFOLLOW_LINKS);
        for (PosixFilePermission p:perms) if(p.name().startsWith("GROUP_")||p.name().startsWith("OTHERS_")) throw new IllegalArgumentException("owner-only file required");
    }
    public void stop(BundleContext context) {
        if (server!=null) try {server.close();} catch(Exception e) { /* shutdown only */ }
    }
}
