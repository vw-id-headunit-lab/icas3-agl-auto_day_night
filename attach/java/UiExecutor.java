package lab.daynight.attach;

import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.concurrent.atomic.AtomicInteger;

/** A queued request can be cancelled; a started mutation is always observed to completion. */
final class UiExecutor {
    interface Dispatcher { void invoke(Runnable task) throws Exception; }
    interface Work { void run() throws Exception; }

    static void execute(Dispatcher dispatcher, final Work work, long queueTimeoutMillis) throws Exception {
        final CountDownLatch finished = new CountDownLatch(1);
        final Throwable[] failure = new Throwable[1];
        final AtomicInteger phase = new AtomicInteger();
        dispatcher.invoke(new Runnable() {
            public void run() {
                if (!phase.compareAndSet(0, 1)) { finished.countDown(); return; }
                try { work.run(); }
                catch (Throwable error) { failure[0] = error; }
                finally { finished.countDown(); }
            }
        });

        boolean interrupted = false;
        try {
            boolean done;
            try { done = finished.await(queueTimeoutMillis, TimeUnit.MILLISECONDS); }
            catch (InterruptedException error) {
                if (phase.compareAndSet(0, 2)) throw error;
                interrupted = true;
                done = false;
            }
            if (!done) {
                if (phase.compareAndSet(0, 2))
                    throw new TimeoutException("UI request expired before starting; no patch changes applied");
                while (finished.getCount() != 0) {
                    try { finished.await(); }
                    catch (InterruptedException error) { interrupted = true; }
                }
            }
            if (failure[0] instanceof Error) throw (Error) failure[0];
            if (failure[0] instanceof Exception) throw (Exception) failure[0];
            if (failure[0] != null) throw new RuntimeException(failure[0]);
        } finally {
            if (interrupted) Thread.currentThread().interrupt();
        }
    }
}
