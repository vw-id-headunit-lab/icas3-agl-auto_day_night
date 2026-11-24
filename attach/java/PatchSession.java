package lab.daynight.attach;

/** UI-thread transaction state; a failed recovery must never look installed. */
final class PatchSession {
    enum State { ORIGINAL, INSTALLING, INSTALLED, RESTORING, RECOVERY_REQUIRED }

    interface Changes {
        void applyDefinitions() throws Exception;
        void applyViews() throws Exception;
        void restoreDefinitions() throws Exception;
        void restoreViews() throws Exception;
    }

    private volatile State state = State.ORIGINAL;

    State state() { return state; }
    boolean installed() { return state == State.INSTALLED; }
    boolean original() { return state == State.ORIGINAL; }

    void requireInstallable() {
        if (!original() && !installed())
            throw new IllegalStateException("Patch state is " + state + "; run remove before installing again");
    }

    void install(Changes changes) throws Exception {
        requireInstallable();
        if (installed()) return;
        state = State.INSTALLING;
        try {
            changes.applyDefinitions();
            changes.applyViews();
            state = State.INSTALLED;
        } catch (Throwable failure) {
            try { restore(changes); }
            catch (Throwable recovery) { failure.addSuppressed(recovery); }
            throwFailure(failure);
        }
    }

    void remove(Changes changes) throws Exception {
        if (original()) return;
        restore(changes);
    }

    private void restore(Changes changes) throws Exception {
        state = State.RESTORING;
        try {
            changes.restoreDefinitions();
            changes.restoreViews();
            state = State.ORIGINAL;
        } catch (Throwable failure) {
            state = State.RECOVERY_REQUIRED;
            throwFailure(failure);
        }
    }

    private static void throwFailure(Throwable failure) throws Exception {
        if (failure instanceof Error) throw (Error) failure;
        if (failure instanceof Exception) throw (Exception) failure;
        throw new RuntimeException(failure);
    }
}
