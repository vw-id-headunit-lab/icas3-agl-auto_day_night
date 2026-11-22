/** Bytecode loading/verification only. Never initialize or execute the HMI. */
public final class VerifyClasses {
    public static void main(String[] args) throws Exception {
        for(String name:args) {
            Class<?> type=Class.forName(name,false,VerifyClasses.class.getClassLoader());
            type.getDeclaredMethods();
            System.out.println("VERIFIED "+name+" from "+type.getProtectionDomain().getCodeSource().getLocation());
        }
    }
}
