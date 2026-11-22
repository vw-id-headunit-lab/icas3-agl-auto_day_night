package lab.vw.daynight;

import de.vw.mib.asl.clientapi.systembaseservices.SystemBaseAPI;
import de.vw.mib.asl.framework.internal.framework.ServiceManager;

/** Compiled against the inspected S2995 jar; proprietary classes are not bundled. */
final class FirmwareRuntime implements QueuedBackend.Runtime {
    private SystemBaseAPI api;
    private final SystemBaseAPI.Listener listener=new SystemBaseAPI.Listener.Adapter();

    public void enqueue(Runnable task) {
        if (ServiceManager.aslThreadSwitch==null) throw new IllegalStateException("ASL thread unavailable");
        ServiceManager.aslThreadSwitch.enqueue(task);
    }
    public boolean ready() {
        if (ServiceManager.aslClientAPIRegistry==null) return false;
        if (api==null) api=ServiceManager.aslClientAPIRegistry.getAPI(SystemBaseAPI.class,0);
        if (api==null||api.getAPIState()!=1) return false;
        // Subscription is idempotently performed once for this API instance.
        if (!subscribed) {api.attributes().subscribe(listener,SystemBaseAPI.Attributes.DayNightValue); subscribed=true;}
        return true;
    }
    private boolean subscribed;
    public void read(final QueuedBackend.Callback callback) {
        api.requests().getDayNightMode(new SystemBaseAPI.Response.Adapter() {
            private Integer value;
            public void getDayNightModeResult(int mode) {value=mode;}
            public void request_finished(int result) {callback.done(result,value);}
        });
    }
    public void write(int desired,final QueuedBackend.Callback callback) {
        api.requests().setDayNightMode(new SystemBaseAPI.Response.Adapter() {
            public void request_finished(int result) {callback.done(result,null);}
        },desired);
    }
    public Boolean daylight() {
        return api.attributes().isDayNightValueValid()?Boolean.valueOf(api.attributes().getDayNightValue()):null;
    }
}
