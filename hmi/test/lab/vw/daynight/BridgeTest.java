package lab.vw.daynight;

import java.io.*;
import java.net.Socket;
import java.util.concurrent.*;

public final class BridgeTest {
    static final class FakeRuntime implements QueuedBackend.Runtime, AutoCloseable {
        final ExecutorService queue=Executors.newSingleThreadExecutor();
        volatile int mode=1,writes;
        volatile boolean holdWrite;
        volatile QueuedBackend.Callback writeCallback;
        public void enqueue(Runnable r){queue.execute(r);}
        public boolean ready(){return true;}
        public void read(QueuedBackend.Callback cb){cb.done(0,mode);}
        public void write(int desired,QueuedBackend.Callback cb){writes++;mode=desired;if(holdWrite)writeCallback=cb;else cb.done(0,null);}
        public Boolean daylight(){return mode==0;}
        public void close(){queue.shutdownNow();}
    }
    static void check(boolean condition,String message){if(!condition)throw new AssertionError(message);}
    static String request(int port,String line) throws Exception {
        try(Socket s=new Socket("127.0.0.1",port)){
            s.setSoTimeout(3000);s.getOutputStream().write((line+"\n").getBytes("US-ASCII"));
            return new BufferedReader(new InputStreamReader(s.getInputStream(),"UTF-8")).readLine();
        }
    }
    public static void main(String[] args)throws Exception{
        try(FakeRuntime f=new FakeRuntime()){
            QueuedBackend b=new QueuedBackend(f,500);
            check(b.set(1,2).ok,"AUTO request failed");
            check(b.status().mode==2&&!b.status().day,"persisted AUTO confused with effective night");
            check(b.set(2,2).ok&&f.writes==1,"equal mode rewritten");
            check(b.set(0,1).code.equals("conflict")&&f.writes==1,"CAS ignored");
        }
        try(FakeRuntime f=new FakeRuntime()){
            final CountDownLatch unblock=new CountDownLatch(1);
            f.enqueue(new Runnable(){public void run(){try{unblock.await();}catch(InterruptedException e){Thread.currentThread().interrupt();}}});
            QueuedBackend b=new QueuedBackend(f,60);
            check(b.set(1,0).code.equals("expired"),"queued request did not expire");
            unblock.countDown();
            check(b.status().mode==1&&f.writes==0,"expired request applied after socket timeout");
        }
        try(FakeRuntime f=new FakeRuntime()){
            f.holdWrite=true;QueuedBackend b=new QueuedBackend(f,100);
            check(b.set(1,2).code.equals("outcome_unknown"),"submitted timeout not classified");
            check(b.set(2,0).code.equals("busy"),"in-flight operation allowed duplicate write");
            f.enqueue(new Runnable(){public void run(){f.writeCallback.done(0,null);}});
            check(b.status().mode==2,"late completion not observable");
        }
        String token="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
        try(FakeRuntime f=new FakeRuntime();BridgeServer s=new BridgeServer(new QueuedBackend(f,500),token,false,0)){
            s.start();check(request(s.port(),"wrong STATUS").contains("unauthorized"),"unauthenticated access");
            check(request(s.port(),token+" STATUS").contains("\"configured_mode\":1"),"status failed");
            check(request(s.port(),token+" SET 1 2").contains("read_only")&&f.writes==0,"read-only gate ignored");
            check(request(s.port(),token+" EXEC java.lang.Runtime").contains("invalid_command"),"arbitrary command accepted");
        }
        try(FakeRuntime f=new FakeRuntime();BridgeServer s=new BridgeServer(new QueuedBackend(f,500),token,true,0)){
            s.start();check(request(s.port(),token+" SET 1 2").contains("\"configured_mode\":2"),"protocol set failed");
            check(f.writes==1,"protocol write count");
        }
        System.out.println("PASS: HMI queue, native AUTO, CAS, no-op, expiry, uncertain completion, loopback protocol and authorization");
    }
}
