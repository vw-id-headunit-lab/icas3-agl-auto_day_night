package generated.de.vw.mib.appui.controlcenter.internal.view;

public final class GeometryHarness {
    public static void main(String[] args) {
        check(GeometryRules.originalHeight("Scm__0", 38, "DropDownDayNightMode") == 280);
        check(GeometryRules.originalHeight("Scm__0", 39, "Background") == 280);
        check(GeometryRules.originalHeight("Scm__0", 40, "DDList") == 237);
        check(GeometryRules.originalHeight("Sc__0", 35, "DDList") == 280);
        check(GeometryRules.originalHeight("Scm__0", 39, "Unrelated280pxWidget") == -1);
        check(GeometryRules.originalHeight("Scm__0", 1, "Background") == -1);
        check(GeometryRules.originalHeight("OtherView", 39, "Background") == -1);
        System.out.println("PASS geometry scope");
    }
    private static void check(boolean ok) { if (!ok) throw new AssertionError(); }
}
