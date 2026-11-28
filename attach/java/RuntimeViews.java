package generated.de.vw.mib.appui.controlcenter.internal.view;

import java.lang.reflect.Field;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.lang.reflect.Proxy;
import java.util.*;
import de.vw.mib.datapool.LocalDataSet;
import de.vw.mib.timer.Timer;
import de.vw.mib.view.internal.tree.AbstractNodeController;
import de.vw.mib.view.internal.tree.CloneData;
import de.vw.mib.widgets.models.WidgetModel;
import org.osgi.framework.*;

/** All widget reads and changes run on the original UI dispatcher. */
public final class RuntimeViews {
    private static final int DAY_NIGHT_PROPERTY = GeometryRules.DAY_NIGHT_PROPERTY;
    private static final Map<WidgetModel, int[]> heights = new WeakHashMap<>();
    private static final Set<WidgetModel> revealedRows = Collections.newSetFromMap(new WeakHashMap<WidgetModel, Boolean>());
    private static final Map<AbstractNodeController, DataSetBinding> dataSets = new WeakHashMap<>();
    private static Field dataSetField;
    private static Timer monitor;
    private static Throwable monitorFailure;

    private static final class DataSetBinding {
        final LocalDataSet original, projection;

        DataSetBinding(LocalDataSet original) {
            this.original = original;
            projection = (LocalDataSet) Proxy.newProxyInstance(
                LocalDataSet.class.getClassLoader(), new Class<?>[]{LocalDataSet.class}, (proxy, method, args) -> {
                    if (method.getName().equals("getLocalHmiInt"))
                        return DayNightSelection.read(original, (Integer) args[0]);
                    try { return method.invoke(original, args); }
                    catch (InvocationTargetException error) { throw error.getCause(); }
                });
        }
    }

    public static int refresh(boolean apply) throws Exception {
        if (!apply) return restore();
        monitorFailure = null;
        dataSetField = AbstractNodeController.class.getDeclaredField("localDataSet");
        dataSetField.setAccessible(true);
        int count = scan(true);
        monitor = de.vw.mib.widgets.internal.ServiceManager.timerManager.createTimer(
            "BetterMebDayNightMenu", 500, true, RuntimeViews::update,
            de.vw.mib.widgets.internal.ServiceManager.eventDispatcher);
        monitor.start();
        return count;
    }

    public static void checkMonitor() {
        if (monitorFailure != null) throw new IllegalStateException("Day/night menu update stopped", monitorFailure);
    }

    private static void update() {
        if (monitor == null) return;
        try { scan(false); }
        catch (Exception | LinkageError error) {
            monitorFailure = error;
            System.err.println("DAYNIGHT_ATTACH menu update failed: " + error);
            try { restore(); }
            catch (Exception | LinkageError restoreError) {
                System.err.println("DAYNIGHT_ATTACH menu recovery failed: " + restoreError);
            }
        }
    }

    private static int scan(boolean initial) throws Exception {
        BundleContext context = de.vw.mib.widgets.internal.ServiceManager.bundleContext;
        if (context == null) throw new IllegalStateException("Original widget service unavailable");
        ServiceReference reference = context.getServiceReference("de.vw.mib.desktop.DesktopManager");
        if (reference == null) throw new IllegalStateException("Original desktop service unavailable");
        try {
            List<AbstractNodeController> nodes = ViewNodes.collect(context.getService(reference), initial);
            validateGeometry(nodes);
            for (AbstractNodeController node : nodes) {
                if (GeometryRules.mode(node.getClass().getSimpleName()) == -1) resize(node);
                else refreshSelection(node);
            }
            return nodes.size();
        } finally { context.ungetService(reference); }
    }

    private static void validateGeometry(List<AbstractNodeController> nodes) {
        for (AbstractNodeController node : nodes) {
            String name = node.getClass().getSimpleName();
            if (GeometryRules.mode(name) != -1) continue;
            CloneData master = node.getCloneMaster();
            Set<String> matched = new HashSet<String>();
            for (int i = 0; i < master.widgetCount(); i++) {
                WidgetModel widget = master.widget(i);
                int original = GeometryRules.originalHeight(name, i, widget.get_name());
                if (original < 0) continue;
                if (!matched.add(widget.get_name())) throw new IllegalStateException("Duplicate dropdown widget");
            }
            if (matched.size() != GeometryRules.widgetCount(name)) throw new IllegalStateException("Incomplete pinned dropdown: " + matched);
        }
    }

    private static void resize(AbstractNodeController node) {
        String name = node.getClass().getSimpleName();
        CloneData master = node.getCloneMaster();
        for (int i = 0; i < master.widgetCount(); i++) {
            WidgetModel widget = master.widget(i);
            int original = GeometryRules.originalHeight(name, i, widget.get_name());
            if (original < 0) continue;
            int current = widget.get_height();
            int patched = GeometryRules.patchedHeight(name, i, widget.get_name());
            // Opening and closing animate through intermediate heights.
            if (current != original) continue;
            heights.putIfAbsent(widget, new int[]{original, patched});
            widget.set_height(patched);
            widget.repaint();
        }
    }

    private static void refreshSelection(AbstractNodeController node) throws Exception {
        String name = node.getClass().getSimpleName();
        LocalDataSet current = (LocalDataSet) dataSetField.get(node);
        if (current == null) return;
        DataSetBinding binding = dataSets.get(node);
        boolean newBinding = binding == null || binding.projection != current;
        if (newBinding) {
            binding = new DataSetBinding(current);
            dataSetField.set(node, binding.projection);
            dataSets.put(node, binding);
            DayNightSelection.observe(node);
        }
        int mode = GeometryRules.mode(name);
        if (mode >= 0) {
            for (CloneData clone : node.getCloneMaster()) {
                WidgetModel row = clone.widget(0);
                DayNightSelection.bind(null, row.getController(), mode);
                if (mode == 2 && !row.is_visible()) {
                    revealedRows.add(row);
                    row.set_visible(true);
                    row.repaint();
                }
            }
        }
        if (newBinding) redraw(node);
    }

    private static void redraw(AbstractNodeController node) throws Exception {
        Method method = node.getClass().getDeclaredMethod("onHmiLocalDatapoolPropertyChanged", int.class);
        method.setAccessible(true);
        method.invoke(node, DAY_NIGHT_PROPERTY);
    }

    private static int restore() throws Exception {
        Timer running = monitor;
        monitor = null;
        if (running != null) running.stop();
        DayNightSelection.shutdown();
        for (Map.Entry<WidgetModel, int[]> entry : heights.entrySet()) {
            WidgetModel widget = entry.getKey();
            int[] dimensions = entry.getValue();
            if (widget.get_height() == dimensions[1]) {
                widget.set_height(dimensions[0]);
                widget.repaint();
            }
        }
        heights.clear();
        for (WidgetModel widget : revealedRows) {
            if (!widget.is_visible()) continue;
            widget.set_visible(false);
            widget.repaint();
        }
        revealedRows.clear();
        int count = dataSets.size();
        Iterator<Map.Entry<AbstractNodeController, DataSetBinding>> iterator = dataSets.entrySet().iterator();
        while (iterator.hasNext()) {
            Map.Entry<AbstractNodeController, DataSetBinding> entry = iterator.next();
            AbstractNodeController node = entry.getKey();
            DataSetBinding binding = entry.getValue();
            if (dataSetField.get(node) == binding.projection) {
                dataSetField.set(node, binding.original);
                if (node.getCloneMaster() != null) {
                    try { redraw(node); }
                    catch (InvocationTargetException error) {
                        System.err.println("DAYNIGHT_ATTACH restored menu redraw failed: " + error.getCause());
                    }
                }
            }
            iterator.remove();
        }
        return count;
    }
}
