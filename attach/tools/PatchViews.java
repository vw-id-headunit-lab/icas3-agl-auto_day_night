import java.nio.file.*;
import jdk.internal.org.objectweb.asm.*;
import jdk.internal.org.objectweb.asm.tree.*;
public final class PatchViews {
 public static void main(String[] args)throws Exception {
  if(args.length==0||args.length%6!=0)throw new IllegalArgumentException("Expected input/output/read-count/mode/property/helper-prefix groups");
  for(int start=0;start<args.length;start+=6)patch(java.util.Arrays.copyOfRange(args,start,start+6));
 }
 private static void patch(String[] a)throws Exception {
  int expected=Integer.parseInt(a[2]),mode=Integer.parseInt(a[3]),property=Integer.parseInt(a[4]);
  String helper=a[5]+"DayNightSelection";
  ClassReader r=new ClassReader(Files.readAllBytes(Paths.get(a[0])));
  ClassNode node=new ClassNode(Opcodes.ASM7);r.accept(node,0);int count=0;
  for(MethodNode m:node.methods) for(AbstractInsnNode i=m.instructions.getFirst();i!=null;i=i.getNext()){
   if(!(i instanceof MethodInsnNode))continue;
   MethodInsnNode call=(MethodInsnNode)i;
   AbstractInsnNode prev=i.getPrevious();while(prev!=null&&prev.getOpcode()<0)prev=prev.getPrevious();
   if(call.getOpcode()==Opcodes.INVOKEINTERFACE&&call.owner.equals("de/vw/mib/datapool/LocalDataSet")&&call.name.equals("getLocalHmiInt")&&call.desc.equals("(I)I")&&prev instanceof LdcInsnNode&&Integer.valueOf(property).equals(((LdcInsnNode)prev).cst)){
    m.instructions.set(call,new MethodInsnNode(Opcodes.INVOKESTATIC,helper,"read","(Lde/vw/mib/datapool/LocalDataSet;I)I",false));count++;i=prev;
   }
  }
  if(count!=expected)throw new IllegalStateException("Expected "+expected+" day/night reads, found "+count);
  int observerHooks=0;
  for(MethodNode m:node.methods){
   String action=m.name.equals("registerAndPerformHmiDpPropertyListeners")?"observe":m.name.equals("deregisterHmiDpPropertyListeners")?"unobserve":null;
   if(action==null||!m.desc.equals("()V"))continue;
   int returns=0;
   for(AbstractInsnNode i=m.instructions.getFirst();i!=null;i=i.getNext())if(i.getOpcode()==Opcodes.RETURN){
    InsnList hook=new InsnList();hook.add(new VarInsnNode(Opcodes.ALOAD,0));
    hook.add(new MethodInsnNode(Opcodes.INVOKESTATIC,helper,action,"(Ljava/lang/Object;)V",false));
    m.instructions.insertBefore(i,hook);m.maxStack=Math.max(m.maxStack,1);returns++;
   }
   if(returns!=1)throw new IllegalStateException("Unexpected observer lifecycle");observerHooks++;
  }
  if(observerHooks!=2)throw new IllegalStateException("Missing observer lifecycle hooks");
  if(mode>=0){
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
      hook.add(new InsnNode(Opcodes.ICONST_0+mode));
     }
     hook.add(new MethodInsnNode(Opcodes.INVOKESTATIC,helper,action,action.equals("bind")?"(Lde/vw/mib/datapool/LocalDataSet;Lde/vw/mib/widgets/controller/WidgetController;I)V":"(Lde/vw/mib/datapool/LocalDataSet;Lde/vw/mib/widgets/controller/WidgetController;)V",false));
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
