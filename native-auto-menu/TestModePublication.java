import java.lang.reflect.*;
import de.vw.mib.asl.internal.system.AslTargetSystemGeneral;
import de.vw.mib.asl.internal.system.SystemServices;
import de.vw.mib.asl.internal.system.clientapi.SystemBaseAPIServiceImpl;

/** Host-only test of actual firmware methods with service dependencies replaced.
 * Constructors and startup are never run; no native services or vehicle I/O.
 */
public final class TestModePublication {
    static int saved=-1,published=-1,has=-1;
    static Boolean effectiveDay;
    public static final class Recorder extends SystemBaseAPIServiceImpl {
        public Recorder(){super((SystemServices)null);}
        public void updateDayNightMode(int value){published=value;}
        public void updateDayNightValue(boolean value){effectiveDay=value;}
        public void updateMapDayNightMode(int value){}
        public void updateMapDayNightValue(boolean value){}
    }
    static Object allocate(Class<?> type)throws Exception {
        Class<?> klass=Class.forName("sun.misc.Unsafe");
        Field field=klass.getDeclaredField("theUnsafe");field.setAccessible(true);
        return klass.getMethod("allocateInstance",Class.class).invoke(field.get(null),type);
    }
    static void set(Object owner,String name,Object value)throws Exception {
        Class<?> type=owner instanceof Class?(Class<?>)owner:owner.getClass();
        while(type!=null){try{Field f=type.getDeclaredField(name);f.setAccessible(true);f.set(owner instanceof Class?null:owner,value);return;}catch(NoSuchFieldException e){type=type.getSuperclass();}}
        throw new NoSuchFieldException(name);
    }
    static Object proxy(String name,final String capture)throws Exception {
        Class<?> type=Class.forName(name);
        return Proxy.newProxyInstance(type.getClassLoader(),new Class<?>[]{type},new InvocationHandler(){
            public Object invoke(Object p,Method m,Object[] args){
                if(capture.equals("persist")&&m.getName().equals("setDayNightMode"))saved=(Integer)args[0];
                if(capture.equals("has")&&m.getName().equals("setPropertyDayNightState"))has=(Integer)args[0];
                Class<?> r=m.getReturnType();if(r==boolean.class)return false;if(r==int.class)return 0;if(r==long.class)return 0L;
                return null;
            }
        });
    }
    static AslTargetSystemGeneral target(boolean night)throws Exception {
        AslTargetSystemGeneral t=(AslTargetSystemGeneral)allocate(AslTargetSystemGeneral.class);
        set(t,"logger",proxy("de.vw.mib.log4mib.Logger","logger"));
        set(t,"configManagerDiag",proxy("de.vw.mib.configuration.ConfigurationManagerDiag","config"));
        set(t,"jIK",proxy("de.vw.mib.asl.systemcommon.persistence.SystemCommon","persist"));
        set(t,"jIM",allocate(Recorder.class));set(t,"jIA",night);
        set(Class.forName("de.vw.mib.asl.internal.system.has.SystemHasBridgeImpl"),"jOr",proxy("de.vw.mib.asl.internal.system.has.SystemHasBridge","has"));
        return t;
    }
    static void check(boolean ok,String why){if(!ok)throw new AssertionError(why);}
    public static void main(String[] args)throws Exception {
        boolean patched=args[0].equals("patched");int checks=0;
        for(boolean night:new boolean[]{false,true}) for(int mode:new int[]{0,1,2}){
            AslTargetSystemGeneral t=target(night);saved=published=has=-1;effectiveDay=null;
            t.setDayNightMode(mode);
            int actual=mode==2?(night?1:0):mode;
            check(saved==mode,"persisted mode changed");
            check(published==(patched?mode:actual),"unexpected menu mode "+published);
            check(effectiveDay!=null&&effectiveDay==(actual==0),"effective theme changed");
            check(has==actual,"HAS state changed");checks++;
            if(mode==2){
                t.dsiGeneralVehicleStatesUpdateDisplayDayNightDesign(!night,1);
                check(saved==2,"AUTO lost after sensor update");
                check(published==(patched?2:(!night?1:0)),"AUTO menu state after sensor change");
                check(effectiveDay==night,"sensor no longer controls effective theme");checks++;
            }
        }
        System.out.println("PASS "+args[0]+": "+checks+" actual firmware scenarios; configured mode, published menu mode, effective theme, vehicle signal and HAS state checked");
    }
}
