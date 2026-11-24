package generated.de.vw.mib.appui.controlcenter.internal.view;

import java.lang.reflect.Method;
import java.util.*;
import de.vw.mib.view.internal.tree.AbstractNodeController;
import de.vw.mib.view.internal.tree.CloneData;
import de.vw.mib.widgets.models.WidgetModel;
import org.osgi.framework.*;

/** All widget reads and changes run on the original UI dispatcher. */
public final class RuntimeViews {
    private static final int DAY_NIGHT_PROPERTY = GeometryRules.DAY_NIGHT_PROPERTY;
    private static final Map<WidgetModel, Integer> heights = new WeakHashMap<WidgetModel, Integer>();
    private static final Map<WidgetModel, Boolean> visibility = new WeakHashMap<WidgetModel, Boolean>();

    public static void validate() throws Exception {
        BundleContext context = de.vw.mib.widgets.internal.ServiceManager.bundleContext;
        if (context == null) throw new IllegalStateException("Original widget service unavailable");
        ServiceReference reference = context.getServiceReference("de.vw.mib.desktop.DesktopManager");
        if (reference == null) throw new IllegalStateException("Original desktop service unavailable");
        try { validateGeometry(ViewNodes.collect(context.getService(reference))); }
        finally { context.ungetService(reference); }
    }

    public static int refresh(boolean apply) throws Exception {
        BundleContext context = de.vw.mib.widgets.internal.ServiceManager.bundleContext;
        if (context == null) throw new IllegalStateException("Original widget service unavailable");
        ServiceReference reference = context.getServiceReference("de.vw.mib.desktop.DesktopManager");
        if (reference == null) throw new IllegalStateException("Original desktop service unavailable");
        try {
            List<AbstractNodeController> nodes = ViewNodes.collect(context.getService(reference));
            validateGeometry(nodes);
            if (!apply) DayNightSelection.shutdown();
            for (AbstractNodeController node : nodes) {
                if (GeometryRules.mode(node.getClass().getSimpleName()) == -1) resize(node, apply);
                else refreshSelection(node, apply);
            }
            System.out.println("DAYNIGHT_ATTACH UI refresh apply=" + apply + " nodes=" + nodes.size());
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
                int current = widget.get_height();
                if (current != original && current != GeometryRules.patchedHeight(name, i, widget.get_name()))
                    throw new IllegalStateException("Unexpected geometry: " + widget.get_name() + "=" + current);
            }
            if (matched.size() != GeometryRules.widgetCount(name)) throw new IllegalStateException("Incomplete pinned dropdown: " + matched);
        }
    }

    private static void resize(AbstractNodeController node, boolean apply) {
        CloneData master = node.getCloneMaster();
        for (int i = 0; i < master.widgetCount(); i++) {
            WidgetModel widget = master.widget(i);
            int original = GeometryRules.originalHeight(node.getClass().getSimpleName(), i, widget.get_name());
            if (original < 0) continue;
            if (apply) {
                if (!heights.containsKey(widget)) heights.put(widget, widget.get_height());
                widget.set_height(GeometryRules.patchedHeight(node.getClass().getSimpleName(), i, widget.get_name()));
            } else {
                widget.set_height(heights.containsKey(widget) ? heights.remove(widget) : original);
            }
            widget.repaint();
        }
    }

    private static void refreshSelection(AbstractNodeController node, boolean apply) throws Exception {
        String name = node.getClass().getSimpleName();
        if (apply) DayNightSelection.observe(node);
        int mode = GeometryRules.mode(name);
        if (mode >= 0) {
            for (CloneData clone : node.getCloneMaster()) {
                WidgetModel row = clone.widget(0);
                if (apply) DayNightSelection.bind(null, row.getController(), mode);
                if (mode == 2) {
                    if (apply) {
                        if (!visibility.containsKey(row)) visibility.put(row, row.is_visible());
                        row.set_visible(true);
                    } else {
                        row.set_visible(visibility.containsKey(row) ? visibility.remove(row) : false);
                    }
                    row.repaint();
                }
            }
        }
        Method update = node.getClass().getDeclaredMethod("onHmiLocalDatapoolPropertyChanged", int.class);
        update.setAccessible(true);
        update.invoke(node, DAY_NIGHT_PROPERTY);
    }
}
