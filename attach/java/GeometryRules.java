package generated.de.vw.mib.appui.controlcenter.internal.view;

/** Only the three pinned day/night dropdown widgets may change dimensions. */
final class GeometryRules {
    static int originalHeight(String node, int index, String widget) {
        int first;
        if ("Sc__0".equals(node)) first = 33;
        else if ("Scm__0".equals(node)) first = 38;
        else return -1;
        if (index == first && "DropDownDayNightMode".equals(widget)) return 280;
        if (index == first + 1 && "Background".equals(widget)) return 280;
        if (index == first + 2 && "DDList".equals(widget)) return "Scm__0".equals(node) ? 237 : 280;
        return -1;
    }

    static boolean targetNode(String simple) {
        return simple.equals("Sc__0") || simple.equals("Sc__1") || simple.equals("Sc__3")
            || simple.equals("Sc__4") || simple.equals("Sc__5") || simple.equals("Scm__0")
            || simple.equals("Scm__1") || simple.equals("Scm__7") || simple.equals("Scm__8")
            || simple.equals("Scm__9");
    }
}
