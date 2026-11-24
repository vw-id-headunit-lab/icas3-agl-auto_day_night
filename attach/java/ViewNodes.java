package generated.de.vw.mib.appui.controlcenter.internal.view;

import java.lang.reflect.Field;
import java.lang.reflect.Modifier;
import java.util.*;
import de.vw.mib.view.internal.tree.AbstractNodeController;
import de.vw.mib.view.internal.tree.CloneData;

/** Bounded discovery, separate from mutations; reject incomplete layout snapshots. */
final class ViewNodes {
    private static final String PREFIX = GeometryRules.CLASS_PREFIX;
    private final IdentityHashMap<Object, Boolean> seen = new IdentityHashMap<Object, Boolean>();
    private final List<AbstractNodeController> nodes = new ArrayList<AbstractNodeController>();

    static List<AbstractNodeController> collect(Object desktop) throws Exception {
        ViewNodes tree = new ViewNodes();
        tree.walk(desktop, 0);
        for (String[] layout : GeometryRules.layouts()) {
            Set<String> expected = new HashSet<String>(Arrays.asList(layout));
            Set<String> found = new HashSet<String>();
            for (AbstractNodeController node : tree.nodes) {
                String name = node.getClass().getSimpleName();
                if (expected.contains(name)) found.add(name);
            }
            if (!found.isEmpty() && !found.equals(expected))
                throw new IllegalStateException("Incomplete control-center layout: " + found);
        }
        if (tree.nodes.isEmpty()) throw new IllegalStateException("Open the original control center and retry");
        return tree.nodes;
    }

    private void walk(Object object, int depth) throws Exception {
        if (object == null || depth > 18 || seen.size() >= 20000 || seen.put(object, Boolean.TRUE) != null) return;
        Class<?> type = object.getClass();
        String name = type.getName();
        if (name.startsWith(PREFIX) && GeometryRules.targetNode(type.getSimpleName()) && object instanceof AbstractNodeController) {
            AbstractNodeController node = (AbstractNodeController)object;
            if (node.getCloneMaster() != null) nodes.add(node);
            walk(node.getCloneMaster(), depth + 1);
        } else if (object instanceof CloneData) {
            CloneData master = (CloneData)object;
            walk(master.nodeController, depth + 1);
            walk(master.subNodeControllers, depth + 1);
            for (CloneData clone : master) if (clone != master) walk(clone, depth + 1);
        } else if (object instanceof Map) {
            for (Object value : ((Map<?, ?>)object).values()) walk(value, depth + 1);
        } else if (object instanceof Iterable) {
            for (Object value : (Iterable<?>)object) walk(value, depth + 1);
        } else if (type.isArray() && !type.getComponentType().isPrimitive()) {
            for (Object value : (Object[])object) walk(value, depth + 1);
        } else if (name.startsWith("de.vw.mib.desktop.") || name.startsWith("de.vw.mib.view.internal.")
                || name.startsWith("generated.de.vw.mib.view.")) {
            for (Class<?> current = type; current != null && current != Object.class; current = current.getSuperclass()) {
                for (Field field : current.getDeclaredFields()) {
                    if (Modifier.isStatic(field.getModifiers()) || field.getType().isPrimitive()) continue;
                    field.setAccessible(true);
                    walk(field.get(object), depth + 1);
                }
            }
        }
    }
}
