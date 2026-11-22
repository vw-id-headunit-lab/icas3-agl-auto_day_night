package lab.vw.daynight;

import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

/** All HMI work is enqueued; the socket thread alone waits for callbacks. */
final class QueuedBackend implements Backend {
    interface Callback { void done(int result, Integer configuredMode); }
    interface Runtime {
        void enqueue(Runnable task);
        boolean ready();
        void read(Callback callback);
        void write(int desired, Callback callback);
        Boolean daylight();
    }
    private final Runtime runtime;
    private final long timeoutMillis;
    private final AtomicBoolean writing=new AtomicBoolean();

    QueuedBackend(Runtime runtime, long timeoutMillis) {
        this.runtime=runtime; this.timeoutMillis=timeoutMillis;
    }
    public Reply status() { return call(null,null); }
    public Reply set(int expected,int desired) {
        if (expected<0||expected>2||desired<0||desired>2) return Reply.error("invalid_command");
        if (!writing.compareAndSet(false,true)) return Reply.error("busy");
        return call(expected,desired);
    }
    private final class Call {
        final CountDownLatch latch=new CountDownLatch(1);
        final long deadline=System.nanoTime()+TimeUnit.MILLISECONDS.toNanos(timeoutMillis);
        final boolean mutation;
        boolean cancelled,submitted,finished;
        volatile Reply reply;
        Call(boolean mutation) { this.mutation=mutation; }
        synchronized boolean expired() { return cancelled||System.nanoTime()>=deadline; }
        synchronized boolean submit() {
            if (expired()) return false;
            submitted=true; return true;
        }
        synchronized boolean cancel() { cancelled=true; return submitted; }
        synchronized void finish(Reply result) {
            if (finished) return;
            finished=true; reply=result;
            if (mutation) writing.set(false);
            latch.countDown();
        }
    }
    private Reply call(final Integer expected,final Integer desired) {
        final Call c=new Call(desired!=null);
        try {
            runtime.enqueue(new Runnable() { public void run() {
                try {
                    if (c.expired()) {c.finish(Reply.error("expired")); return;}
                    if (!runtime.ready()) {c.finish(Reply.error("not_ready")); return;}
                    runtime.read(new Callback() {public void done(int result,Integer current) {
                        try {
                            if (result!=0||current==null||current<0||current>2) {c.finish(Reply.error("not_ready")); return;}
                            if (desired==null) {c.finish(new Reply(true,"ok",current,runtime.daylight(),false)); return;}
                            if (!current.equals(expected)) {c.finish(Reply.error("conflict")); return;}
                            if (current.equals(desired)) {c.finish(new Reply(true,"ok",current,runtime.daylight(),false)); return;}
                            if (!c.submit()) {c.finish(Reply.error("expired")); return;}
                            runtime.write(desired,new Callback() {public void done(int code,Integer unused) {
                                if (code!=0) {c.finish(Reply.error("native_result_"+code)); return;}
                                // Read back persisted configuration, never infer success from an effective 0/1 attribute.
                                try {runtime.read(new Callback() {public void done(int rc,Integer mode) {
                                    try {
                                        if (rc!=0||!desired.equals(mode)) c.finish(Reply.error("verify_failed"));
                                        else c.finish(new Reply(true,"ok",mode,runtime.daylight(),true));
                                    } catch (RuntimeException|LinkageError e) {c.finish(Reply.error("outcome_unknown"));}
                                }});} catch (RuntimeException|LinkageError e) {c.finish(Reply.error("outcome_unknown"));}
                            }});
                        } catch (RuntimeException|LinkageError e) {c.finish(Reply.error("outcome_unknown"));}
                    }});
                } catch (RuntimeException|LinkageError e) {c.finish(Reply.error("not_ready"));}
            }});
            if (c.latch.await(timeoutMillis,TimeUnit.MILLISECONDS)) return c.reply;
            return Reply.error(c.cancel()?"outcome_unknown":"expired");
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt(); return Reply.error(c.cancel()?"outcome_unknown":"expired");
        } catch (RuntimeException|LinkageError e) {
            c.finish(Reply.error("not_ready")); return c.reply;
        }
    }
}
