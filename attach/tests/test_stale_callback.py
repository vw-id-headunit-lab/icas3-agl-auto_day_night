"""Compile the real menu adapter with tiny APIs, then delay native callbacks deliberately."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
JDK = os.environ.get('DAYNIGHT_TEST_JDK')
PREFIX = 'generated/de/vw/mib/appui/controlcenter/internal/view/'

STUBS = {
    'de/vw/mib/datapool/LocalDataSet.java': 'public interface LocalDataSet { int getLocalHmiInt(int property); }',
    'de/vw/mib/widgets/controller/WidgetController.java': 'public interface WidgetController { void addOutEventListener(de.vw.mib.widgets.event.OutEventListener listener); void removeOutEventListener(de.vw.mib.widgets.event.OutEventListener listener); }',
    'de/vw/mib/widgets/event/OutEventListener.java': 'public interface OutEventListener {}',
    'de/vw/mib/timer/Invoker.java': 'public interface Invoker { void invoke(Runnable task); }',
    'de/vw/mib/timer/Timer.java': 'public interface Timer { void stop(); void start(); void setRunnable(Runnable task,Invoker invoker); }',
    'de/vw/mib/timer/TimerManager.java': 'public interface TimerManager { Timer createTimer(String name,int delay,boolean repeat,Runnable task,Invoker invoker); }',
    'de/vw/mib/widgets/internal/ServiceManager.java': 'public class ServiceManager { public static de.vw.mib.timer.Invoker eventDispatcher; public static de.vw.mib.timer.TimerManager timerManager; }',
    'de/vw/mib/asl/clientapi/systembaseservices/SystemBaseAPI.java': '''public interface SystemBaseAPI {
        int getAPIState(); Requests requests();
        interface Requests { void getDayNightMode(Response.Adapter response); void setDayNightMode(Response.Adapter response,int value); }
        interface Response { class Adapter { public void getDayNightModeResult(int value) {} public void request_finished(int result) {} } }
    }''',
    'de/vw/mib/asl/framework/internal/framework/ServiceManager.java': '''public class ServiceManager {
        public interface Switch { void enqueue(Runnable task); }
        public interface Registry { <T> T getAPI(Class<T> type,int instance); }
        public static Switch aslThreadSwitch; public static Registry aslClientAPIRegistry;
    }''',
}

HARNESS = r'''
import java.lang.reflect.*;
import java.util.*;
import de.vw.mib.asl.clientapi.systembaseservices.SystemBaseAPI;
import de.vw.mib.asl.framework.internal.framework.ServiceManager;
import generated.de.vw.mib.appui.controlcenter.internal.view.DayNightSelection;
public class StaleHarness {
    static int writes;
    static final List<SystemBaseAPI.Response.Adapter> callbacks=new ArrayList<>();
    public static class View { public void onHmiLocalDatapoolPropertyChanged(int property) {} }
    static void field(String name,Object value)throws Exception {Field f=DayNightSelection.class.getDeclaredField(name);f.setAccessible(true);f.set(null,value);}
    static void fetch()throws Exception {
        field("requesting",false);
        Method m=DayNightSelection.class.getDeclaredMethod("fetch",int.class,long.class);m.setAccessible(true);m.invoke(null,0,42L);
    }
    public static void main(String[] args)throws Exception {
        ServiceManager.aslThreadSwitch=Runnable::run;
        de.vw.mib.widgets.internal.ServiceManager.eventDispatcher=Runnable::run;
        final SystemBaseAPI api=new SystemBaseAPI(){
            public int getAPIState(){return 1;}
            public Requests requests(){return new Requests(){
                public void getDayNightMode(Response.Adapter response){callbacks.add(response);}
                public void setDayNightMode(Response.Adapter response,int value){writes++;}
            };}
        };
        ServiceManager.aslClientAPIRegistry=new ServiceManager.Registry(){public <T>T getAPI(Class<T> type,int i){return type.cast(api);}};
        DayNightSelection.observe(new View());field("latestClick",42L);fetch();
        SystemBaseAPI.Response.Adapter old=callbacks.get(callbacks.size()-1);
        if(args[0].equals("uninstall"))DayNightSelection.shutdown();else fetch();
        old.getDayNightModeResult(2);old.request_finished(0);
        if(writes!=0)throw new AssertionError("stale request changed mode: "+writes);
        System.out.println("PASS stale "+args[0]);
    }
}
'''


@unittest.skipUnless(JDK and (Path(JDK)/'bin/javac').is_file(), 'Set DAYNIGHT_TEST_JDK')
class StaleCallbackTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(prefix='daynight-stale-test-')
        cls.root = Path(cls.temp.name)
        cls.java = Path(JDK)/'bin'
        sources = []
        for relative, body in STUBS.items():
            path = cls.root/relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text('package '+relative.rsplit('/', 1)[0].replace('/', '.')+';\n'+body)
            sources.append(str(path))
        (cls.root/'StaleHarness.java').write_text(HARNESS)
        subprocess.run([str(cls.java/'javac'), '--release', '8', '-d', str(cls.root), *sources,
                        str(ROOT/'menu/java'/PREFIX/'DayNightSelection.java'), str(cls.root/'StaleHarness.java')],
                       check=True, capture_output=True)

    @classmethod
    def tearDownClass(cls):
        cls.temp.cleanup()

    def run_scenario(self, action):
        result = subprocess.run([str(self.java/'java'), '-cp', str(self.root), 'StaleHarness', action],
                                capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_callback_after_uninstall_never_writes_mode(self):
        self.run_scenario('uninstall')

    def test_superseded_callback_never_writes_mode(self):
        self.run_scenario('superseded')
