package lab.daynight.attach;

import java.io.IOException;
import java.nio.file.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.*;

/** Failure injection against production transaction and queue code, no vendor classes. */
public final class ReviewHarness {
    static void check(boolean ok) { if (!ok) throw new AssertionError(); }

    static final class Changes implements PatchSession.Changes {
        int apply, restore;
        boolean failApply, failRestore, errorApply;
        public void applyDefinitions() { apply++; }
        public void applyViews() throws Exception {
            if (errorApply) throw new LinkageError("view link failed");
            if (failApply) throw new IOException("view refresh failed");
        }
        public void restoreDefinitions() { restore++; }
        public void restoreViews() throws Exception { if (failRestore) throw new IOException("restore failed"); }
    }

    static void rollbackFailure() throws Exception {
        PatchSession session = new PatchSession();
        Changes changes = new Changes(); changes.failApply = true; changes.failRestore = true;
        try { session.install(changes); throw new AssertionError(); }
        catch (IOException expected) { check(expected.getSuppressed().length == 1); }
        check(session.state() == PatchSession.State.RECOVERY_REQUIRED);
        check(!session.installed());
        try { session.install(changes); throw new AssertionError(); }
        catch (IllegalStateException expected) { check(changes.apply == 1); }
        changes.failRestore = false;
        session.remove(changes);
        check(session.original());
        changes.failApply = false;
        session.install(changes);
        check(session.installed());
        session.install(changes); check(changes.apply == 2);
    }

    static void linkageRollback() throws Exception {
        PatchSession session = new PatchSession(); Changes changes = new Changes(); changes.errorApply = true;
        try { session.install(changes); throw new AssertionError(); }
        catch (LinkageError expected) { check(changes.restore == 1); check(session.original()); }
    }

    static void queueCancellation() throws Exception {
        final Runnable[] pending = new Runnable[1]; final AtomicInteger applied = new AtomicInteger();
        try { UiExecutor.execute(task -> pending[0] = task, () -> applied.incrementAndGet(), 1); throw new AssertionError(); }
        catch (TimeoutException expected) { }
        pending[0].run(); check(applied.get() == 0);
    }

    static void interruptedMutation() throws Exception {
        CountDownLatch started = new CountDownLatch(1), release = new CountDownLatch(1);
        AtomicBoolean finished = new AtomicBoolean(), interruptPreserved = new AtomicBoolean();
        AtomicReference<Throwable> failure = new AtomicReference<Throwable>();
        Thread caller = new Thread(() -> {
            try {
                UiExecutor.execute(task -> new Thread(task).start(), () -> {
                    started.countDown(); release.await(); finished.set(true);
                }, 5000);
                interruptPreserved.set(Thread.currentThread().isInterrupted());
            } catch (Throwable error) { failure.set(error); }
        });
        caller.start(); check(started.await(5, TimeUnit.SECONDS)); caller.interrupt();
        check(caller.isAlive()); release.countDown(); caller.join(5000);
        check(!caller.isAlive() && finished.get() && interruptPreserved.get() && failure.get() == null);
    }

    static void resultReservation(String directory) throws Exception {
        Path root = Paths.get(directory), path = root.resolve("report.json");
        try (AgentRequest ignored = AgentRequest.open("install|" + path)) {
            try { AgentRequest.open("status|" + path); throw new AssertionError(); }
            catch (FileAlreadyExistsException expected) { }
        }
        check(Files.size(path) == 0);
        try { AgentRequest.open("install|" + root.resolve("missing/report.json")); throw new AssertionError(); }
        catch (NoSuchFileException expected) { }
        check(AgentRequest.quote("\t\b\n").equals("\"\\u0009\\u0008\\u000a\""));
    }

    public static void main(String[] args) throws Exception {
        switch (args[0]) {
            case "rollback": rollbackFailure(); break;
            case "linkage": linkageRollback(); break;
            case "queue": queueCancellation(); break;
            case "interrupt": interruptedMutation(); break;
            case "report": resultReservation(args[1]); break;
            default: throw new IllegalArgumentException();
        }
        System.out.println("PASS " + args[0]);
    }
}
