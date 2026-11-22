package lab.vw.daynight;

interface Backend {
    Reply status();
    Reply set(int expected, int desired);
}

final class Reply {
    final boolean ok;
    final String code;
    final Integer mode;
    final Boolean day;
    final boolean changed;
    Reply(boolean ok, String code, Integer mode, Boolean day, boolean changed) {
        this.ok=ok; this.code=code; this.mode=mode; this.day=day; this.changed=changed;
    }
    static Reply error(String code) { return new Reply(false,code,null,null,false); }
    String json() {
        return "{\"ok\":"+ok+",\"code\":\""+code+"\",\"configured_mode\":"+mode+
            ",\"day\":"+day+",\"changed\":"+changed+"}";
    }
}
