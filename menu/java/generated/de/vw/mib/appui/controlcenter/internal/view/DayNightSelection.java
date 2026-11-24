package generated.de.vw.mib.appui.controlcenter.internal.view;
import de.vw.mib.datapool.LocalDataSet;
import de.vw.mib.asl.clientapi.systembaseservices.SystemBaseAPI;
import de.vw.mib.widgets.controller.WidgetController;
import de.vw.mib.widgets.event.OutEventListener;
import de.vw.mib.timer.Timer;
import de.vw.mib.timer.Invoker;
import java.lang.reflect.Method;
import java.util.Map;
import java.util.WeakHashMap;
/** Menu projection only. All maps/cache are UI-thread confined; reads run on ASL. */
public final class DayNightSelection implements OutEventListener,Runnable {
 private static final java.util.concurrent.atomic.AtomicInteger logCount=new java.util.concurrent.atomic.AtomicInteger();
 private static void trace(String text){if(logCount.getAndIncrement()<150)System.out.println("DAYNIGHT_MENU "+Thread.currentThread().getName()+" "+text);}
 private static final int PROPERTY=2000000029;
 private static final Map<WidgetController,DayNightSelection> listeners=new WeakHashMap<>();
 private static final Map<Object,Method> views=new WeakHashMap<>();
 private static int saved=-1;
 private static boolean requesting;
 private static long sequence, requestStarted;
 private static volatile long epoch, activeRequest;
 private static Timer timer;
 private static DayNightSelection pending;
 private int remaining;
 private final int wantedMode;
 private long click;
 private static volatile long latestClick;
 private static final java.util.concurrent.atomic.AtomicLong attemptedClick=new java.util.concurrent.atomic.AtomicLong();
 private DayNightSelection(int mode) {wantedMode=mode;}
 private static Invoker ui(){return de.vw.mib.widgets.internal.ServiceManager.eventDispatcher;}
 /** Called on the UI thread when the runtime agent is removed. */
 public static void shutdown(){
  epoch++;requesting=false;latestClick++;pending=null;saved=-1;
  if(timer!=null){timer.stop();timer=null;}
  for(Map.Entry<WidgetController,DayNightSelection> entry:listeners.entrySet())
   entry.getKey().removeOutEventListener(entry.getValue());
  listeners.clear();views.clear();
 }
 private static void redraw(){
  for(Object view:views.keySet().toArray()){
   Method method=views.get(view);
   if(method!=null)try{method.invoke(view,PROPERTY);}catch(ReflectiveOperationException|RuntimeException|LinkageError e){trace("redraw failed "+e);}
  }
 }
 public static void observe(Object view){
  try{
   Method m=view.getClass().getDeclaredMethod("onHmiLocalDatapoolPropertyChanged",int.class);
   m.setAccessible(true);views.put(view,m);trace("observe "+view.getClass().getSimpleName()+" views="+views.size());fetch();
  }catch(ReflectiveOperationException|RuntimeException|LinkageError e){trace("reflection failed "+e);}
 }
 public static void unobserve(Object view){
  views.remove(view);
  if(views.isEmpty()){
   saved=-1;epoch++;requesting=false;latestClick++;
   if(timer!=null)try{timer.stop();}catch(RuntimeException|LinkageError e){}
   pending=null;
  }
 }
 /** Never waits or touches the synchronous persistence service on a rendering thread. */
 public static int read(LocalDataSet local,int property){
  int fallback=local.getLocalHmiInt(property);
  if(property!=PROPERTY)return fallback;
  fetch();return saved>=0?saved:fallback;
 }
 private static void deliver(final long expectedEpoch,final long request,final int value){
  trace("read result="+value+" request="+request);
  try{
   Invoker invoker=ui();
   if(invoker!=null)invoker.invoke(()->{
    trace("UI deliver "+value);if(expectedEpoch!=epoch||request!=activeRequest)return;
    requesting=false;
    int next=value>=0&&value<=2?value:-1;
    if(next!=saved){saved=next;redraw();}
   });
  }catch(RuntimeException|LinkageError e){trace("deliver failed "+e);}
 }
 private static void fetch(){fetch(-1,0);}
 private static void fetch(final int desired,final long click){
  if(views.isEmpty()||ui()==null){trace("fetch unavailable views="+views.size()+" ui="+ui());return;}
  if(requesting&&System.nanoTime()-requestStarted<2000000000L)return;
  try{
   if(de.vw.mib.asl.framework.internal.framework.ServiceManager.aslThreadSwitch==null){trace("no ASL switch");return;}
   trace("enqueue read");requesting=true;requestStarted=System.nanoTime();
   final long expectedEpoch=epoch,request=++sequence;activeRequest=request;
   de.vw.mib.asl.framework.internal.framework.ServiceManager.aslThreadSwitch.enqueue(()->{
    try{
     if(de.vw.mib.asl.framework.internal.framework.ServiceManager.aslClientAPIRegistry==null){deliver(expectedEpoch,request,-1);return;}
     SystemBaseAPI api=de.vw.mib.asl.framework.internal.framework.ServiceManager.aslClientAPIRegistry.getAPI(SystemBaseAPI.class,0);
     if(api==null||api.getAPIState()!=1){deliver(expectedEpoch,request,-1);return;}
     api.requests().getDayNightMode(new SystemBaseAPI.Response.Adapter(){
      private Integer mode;
      public void getDayNightModeResult(int value){mode=value;}
      public void request_finished(int result){
       // A late response must not write after timeout, a newer request, or uninstall.
       if(expectedEpoch!=epoch||request!=activeRequest)return;
       long previousAttempt=attemptedClick.get();
       if(result==0&&mode!=null&&desired>=0&&mode!=desired&&click==latestClick
          &&previousAttempt!=click&&attemptedClick.compareAndSet(previousAttempt,click)){
        // Factory UI can incorrectly discard manual selection equal to effective theme.
        // Only one intervention per explicit click; uncertain writes are never retried.
        trace("explicit-click correction mode="+desired+" saved="+mode);
        api.requests().setDayNightMode(new SystemBaseAPI.Response.Adapter(){
         public void request_finished(int code){
          if(code!=0){deliver(expectedEpoch,request,-1);return;}
          api.requests().getDayNightMode(new SystemBaseAPI.Response.Adapter(){
           private Integer confirmed;
           public void getDayNightModeResult(int value){confirmed=value;}
           public void request_finished(int rc){deliver(expectedEpoch,request,rc==0&&confirmed!=null?confirmed:-1);}
          });
         }
        },desired);
       }else deliver(expectedEpoch,request,result==0&&mode!=null?mode:-1);
      }
     });
    }catch(RuntimeException|LinkageError e){deliver(expectedEpoch,request,-1);}
   });
  }catch(RuntimeException|LinkageError e){requesting=false;trace("enqueue failed "+e);}
 }
 public static void bind(LocalDataSet unused,WidgetController controller,int mode){
  try{
   if(mode<0||mode>2||controller==null||listeners.containsKey(controller))return;
   DayNightSelection listener=new DayNightSelection(mode);controller.addOutEventListener(listener);listeners.put(controller,listener);trace("bind "+controller.getClass().getSimpleName());
  }catch(RuntimeException|LinkageError e){}
 }
 public static void unbind(LocalDataSet unused,WidgetController controller){
  try{DayNightSelection l=listeners.remove(controller);if(l!=null)controller.removeOutEventListener(l);}catch(RuntimeException|LinkageError e){}
 }
 private void release(String name){
  trace("out event "+name);if(!"buttonReleased".equals(name))return;
  try{
   de.vw.mib.timer.TimerManager manager=de.vw.mib.widgets.internal.ServiceManager.timerManager;
   Invoker invoker=ui();if(manager==null||invoker==null)return;
   if(timer!=null)timer.stop();
   if(timer==null)timer=manager.createTimer("DayNightMenuReadback",200,false,this,invoker);
   click=++latestClick;timer.setRunnable(this,invoker);remaining=6;pending=this;timer.start();
  }catch(RuntimeException|LinkageError e){pending=null;trace("timer failed "+e);}
 }
 public void run(){
  trace("tick remaining="+remaining);
  if(pending!=this)return;
  fetch(wantedMode,click);
  try{if(--remaining>0&&!views.isEmpty())timer.start();else pending=null;}catch(RuntimeException|LinkageError e){pending=null;trace("timer failed "+e);}
 }
 public void outHMIEventFired(String n){release(n);}
 public void outIntegerEventFired(String n,int v){release(n);}
 public void outBooleanEventFired(String n,boolean v){release(n);}
 public void outStringEventFired(String n,String v){release(n);}
 public void outPointEventFired(String n,int a,int b,int c,int d){release(n);}
 public void outListItemEventFired(String n,int a,int b,int c,int d,String v,long id){release(n);}
 public void outSpellerEventFired(String n,String v,int a,int b,int c){release(n);}
}
