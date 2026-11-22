package lab.vw.daynight;

import java.io.*;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;

final class BridgeServer implements Closeable {
    private final Backend backend;
    private final byte[] token;
    private final boolean allowWrite;
    private final ServerSocket server;
    private volatile boolean running;
    BridgeServer(Backend backend,String token,boolean allowWrite,int port) throws IOException {
        this.backend=backend; this.token=token.getBytes(StandardCharsets.US_ASCII); this.allowWrite=allowWrite;
        server=new ServerSocket(port,4,InetAddress.getByName("127.0.0.1"));
    }
    int port() {return server.getLocalPort();}
    void start() {
        running=true;
        Thread t=new Thread(new Runnable(){public void run(){serve();}},"vw-daynight-bridge");
        t.setDaemon(true); t.start();
    }
    private void serve() {
        while (running) {
            try (Socket s=server.accept()) {
                s.setSoTimeout(2000);
                if (!s.getInetAddress().isLoopbackAddress()) continue;
                StringBuilder line=new StringBuilder(); InputStream in=s.getInputStream();
                for (int ch;(ch=in.read())!=-1&&ch!='\n';) {
                    if (ch<32||ch>126||line.length()>=160) throw new IOException("invalid request");
                    line.append((char)ch);
                }
                Reply reply=dispatch(line.toString());
                s.getOutputStream().write((reply.json()+"\n").getBytes(StandardCharsets.UTF_8));
            } catch (IOException|RuntimeException|LinkageError e) {
                // Optional component: an unavailable client/backend must not stop the HMI.
            }
        }
    }
    private Reply dispatch(String line) {
        String[] fields=line.split(" ",-1);
        if (fields.length<2||!MessageDigest.isEqual(token,fields[0].getBytes(StandardCharsets.US_ASCII))) return Reply.error("unauthorized");
        if (fields.length==2&&fields[1].equals("STATUS")) return backend.status();
        if (fields.length==4&&fields[1].equals("SET")&&fields[2].matches("[012]")&&fields[3].matches("[012]")) {
            if (!allowWrite) return Reply.error("read_only");
            return backend.set(Integer.parseInt(fields[2]),Integer.parseInt(fields[3]));
        }
        return Reply.error("invalid_command");
    }
    public void close() throws IOException {running=false;server.close();}
}
