import java.nio.file.*;
import jdk.internal.org.objectweb.asm.*;
import jdk.internal.org.objectweb.asm.tree.*;
public final class PatchViews {
 public static void main(String[] a)throws Exception {
  ClassReader r=new ClassReader(Files.readAllBytes(Paths.get(a[0])));
  ClassNode node=new ClassNode(Opcodes.ASM7);r.accept(node,0);int count=0;
  for(MethodNode m:node.methods) for(AbstractInsnNode i=m.instructions.getFirst();i!=null;i=i.getNext()){
   if(!(i instanceof MethodInsnNode))continue;
   MethodInsnNode call=(MethodInsnNode)i;
   AbstractInsnNode prev=i.getPrevious();while(prev!=null&&prev.getOpcode()<0)prev=prev.getPrevious();
   if(call.getOpcode()==Opcodes.INVOKEINTERFACE&&call.owner.equals("de/vw/mib/datapool/LocalDataSet")&&call.name.equals("getLocalHmiInt")&&call.desc.equals("(I)I")&&prev instanceof LdcInsnNode&&Integer.valueOf(2000000029).equals(((LdcInsnNode)prev).cst)){
    m.instructions.set(call,new MethodInsnNode(Opcodes.INVOKESTATIC,"generated/de/vw/mib/appui/controlcenter/internal/view/DayNightSelection","read","(Lde/vw/mib/datapool/LocalDataSet;I)I",false));count++;i=prev;
   }
  }
  int expected=(r.getClassName().endsWith("/Sc__1")||r.getClassName().endsWith("/Scm__1"))?1:2;
  if(count!=expected)throw new IllegalStateException("Expected "+expected+" day/night reads, found "+count);
  int observerHooks=0;
  for(MethodNode m:node.methods){
   String action=m.name.equals("registerAndPerformHmiDpPropertyListeners")?"observe":m.name.equals("deregisterHmiDpPropertyListeners")?"unobserve":null;
   if(action==null||!m.desc.equals("()V"))continue;
   int returns=0;
   for(AbstractInsnNode i=m.instructions.getFirst();i!=null;i=i.getNext())if(i.getOpcode()==Opcodes.RETURN){
    InsnList hook=new InsnList();hook.add(new VarInsnNode(Opcodes.ALOAD,0));
    hook.add(new MethodInsnNode(Opcodes.INVOKESTATIC,"generated/de/vw/mib/appui/controlcenter/internal/view/DayNightSelection",action,"(Ljava/lang/Object;)V",false));
    m.instructions.insertBefore(i,hook);m.maxStack=Math.max(m.maxStack,1);returns++;
   }
   if(returns!=1)throw new IllegalStateException("Unexpected observer lifecycle");observerHooks++;
  }
  if(observerHooks!=2)throw new IllegalStateException("Missing observer lifecycle hooks");
  if(expected==2){
   int hooks=0;
   for(MethodNode m:node.methods){
    String action=m.name.equals("registerOutEvents")?"bind":m.name.equals("deregisterOutEvents")?"unbind":null;
    if(action==null||!m.desc.equals("(Lde/vw/mib/view/internal/tree/CloneData;)V"))continue;
    int returns=0;
    for(AbstractInsnNode i=m.instructions.getFirst();i!=null;i=i.getNext())if(i.getOpcode()==Opcodes.RETURN){
     InsnList hook=new InsnList();
     hook.add(new VarInsnNode(Opcodes.ALOAD,0));
     hook.add(new FieldInsnNode(Opcodes.GETFIELD,node.name,"localDataSet","Lde/vw/mib/datapool/LocalDataSet;"));
     hook.add(new VarInsnNode(Opcodes.ALOAD,1));hook.add(new InsnNode(Opcodes.ICONST_0));
     hook.add(new MethodInsnNode(Opcodes.INVOKEVIRTUAL,"de/vw/mib/view/internal/tree/CloneData","widget","(I)Lde/vw/mib/widgets/models/WidgetModel;",false));
     hook.add(new MethodInsnNode(Opcodes.INVOKEINTERFACE,"de/vw/mib/widgets/models/WidgetModel","getController","()Lde/vw/mib/widgets/controller/WidgetController;",true));
     if(action.equals("bind")){
      String simple=node.name.substring(node.name.lastIndexOf('/')+1);
      int mode=(simple.equals("Sc__3")||simple.equals("Scm__7"))?0:(simple.equals("Sc__4")||simple.equals("Scm__8"))?1:2;
      hook.add(new InsnNode(Opcodes.ICONST_0+mode));
     }
     hook.add(new MethodInsnNode(Opcodes.INVOKESTATIC,"generated/de/vw/mib/appui/controlcenter/internal/view/DayNightSelection",action,action.equals("bind")?"(Lde/vw/mib/datapool/LocalDataSet;Lde/vw/mib/widgets/controller/WidgetController;I)V":"(Lde/vw/mib/datapool/LocalDataSet;Lde/vw/mib/widgets/controller/WidgetController;)V",false));
     m.instructions.insertBefore(i,hook);m.maxStack=Math.max(m.maxStack,3);returns++;
    }
    if(returns!=1)throw new IllegalStateException("Unexpected lifecycle shape");hooks++;
   }
   if(hooks!=2)throw new IllegalStateException("Missing lifecycle hooks");
  }
  ClassWriter w=new ClassWriter(r,0);node.accept(w);
  Files.write(Paths.get(a[1]),w.toByteArray());System.out.println(r.getClassName()+" redirected_daynight_reads="+count);
 }
}
