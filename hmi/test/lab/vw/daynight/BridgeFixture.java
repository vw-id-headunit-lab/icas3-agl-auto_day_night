package lab.vw.daynight;

import java.nio.file.Files;
import java.nio.file.Paths;

/** Host-only integration fixture; deliberately excluded from the bridge jar. */
public final class BridgeFixture {
    public static void main(String[] args)throws Exception{
        String token=new String(Files.readAllBytes(Paths.get(args[0])),"US-ASCII").trim();
        try(BridgeTest.FakeRuntime runtime=new BridgeTest.FakeRuntime();
            BridgeServer server=new BridgeServer(new QueuedBackend(runtime,1000),token,true,0)){
            server.start();System.out.println(server.port());System.out.flush();
            while(System.in.read()!=-1){}
        }
    }
}
