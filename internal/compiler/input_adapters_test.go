package compiler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInputAdaptersPreserveNativeContracts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "input.km")
	input := `import {decodeInput,bindInput} from "kinmokusei/input";
import go json from "encoding/json";
import go bytes from "bytes";
import go io from "io";
import go errors from "errors";
import go url from "net/url";
struct RawChild{public title:bstring;}
struct RawInput{public name:bstring;public children:RawChild[];public optional:*RawChild;public count:int;}
struct Record{public name:string;public titles:string[];}
class Child{constructor(public title:string){}}
class User{constructor(public name:string,public children:Child[],public optional:Child|null,public count:int){}}
let trace=0;
function fresh():RawInput{return RawInput{name:b"",children:[],optional:nil,count:0};}
function read(payload:byte[],strict:boolean):Result<RawInput>{
 trace=trace*10+1;let raw=fresh();
 if(strict){const decoder=json.NewDecoder(bytes.NewReader(payload));decoder.DisallowUnknownFields();const err=decoder.Decode(&raw);if(err!=nil){return fail(err);}let extra:bstring=b"";if(decoder.Decode(&extra)!=io.EOF){return fail(errors.New("trailing JSON value"));}}
 else{const err=json.Unmarshal(payload,&raw);if(err!=nil){return fail(err);}}
 return ok(raw);
}
function construct(raw:RawInput):Result<User>{
 trace=trace*10+2;const name=string(raw.name)?;
 if(name==""||raw.count<0){return fail(errors.New("invalid input"));}
 let children:Child[]=[];
 for(const child of raw.children){const title=string(child.title)?;children=append(children,new Child(title));}
 let optional:Child|null=null;
 if(raw.optional!=nil){const title=string(raw.optional.title)?;optional=new Child(title);}
 return ok(new User(name,children,optional,raw.count));
}
function snapshot(value:User):bstring{const [encoded,err]=jsonSnapshot(value);if(err!=nil){return b"snapshot failed";}return bstring(encoded);}
export function Apply(payload:byte[],strict:boolean):(bstring,bstring,int,boolean){
 trace=0;let target=new User("original",[new Child("kept")],null,9);const alias=target;
 const [err]=bindInput(()=>{return read(payload,strict);},construct,(value:User)=>{target=value;});
 return snapshot(target),snapshot(alias),trace,err==nil;
}
export function StructValue(payload:byte[]):Result<bstring>{
 const value=decodeInput(()=>{return read(payload,false);},(raw:RawInput):Result<Record>=>{
  const name=string(raw.name)?;let titles:string[]=[];
  for(const child of raw.children){const title=string(child.title)?;titles=append(titles,title);}
  return ok(Record{name:name,titles:titles});
 })?;
 const encoded=jsonSnapshot(value)?;return ok(bstring(encoded));
}
export function Query(query:bstring):(bstring,bstring,int,boolean){
 trace=0;let target=new User("original",[new Child("kept")],null,9);const alias=target;
 const [err]=bindInput(():Result<RawInput>=>{
  trace=trace*10+1;const values=url.ParseQuery(query)?;let raw=fresh();raw.name=values.Get("name");return ok(raw);
 },construct,(value:User)=>{target=value;});
 return snapshot(target),snapshot(alias),trace,err==nil;
}
struct CustomRaw{public name:bstring;public pointer function UnmarshalJSON(payload:byte[]):error{this.name=b"\xff";if(len(payload)>0&&payload[0]==byte(116)){return nil;}return errors.New("custom decoder failed after writing");}}
export function CustomFailure(succeed:boolean):boolean{
 let target=new User("original",[],null,9);const alias=target;let constructed=false;
 const [err]=bindInput(():Result<CustomRaw>=>{let raw=CustomRaw{name:b""};let payload:byte[]=[byte(123),byte(125)];if(succeed){payload=[byte(116),byte(114),byte(117),byte(101)];}const err=json.Unmarshal(payload,&raw);if(err!=nil){return fail(err);}return ok(raw);},(raw:CustomRaw):Result<User>=>{constructed=true;const name=string(raw.name)?;return ok(new User(name,[],null,0));},(value:User)=>{target=value;});
 return err!=nil&&constructed==succeed&&target===alias&&target.name=="original";
}
export function InvalidRaw():boolean{
 let target=new User("original",[],null,9);const alias=target;
 const [err]=bindInput(():Result<RawInput>=>{let raw=fresh();raw.name=b"\xff";return ok(raw);},construct,(value:User)=>{target=value;});
 return err!=nil&&target===alias&&target.name=="original";
}
export function PublishOrder():boolean{
 let order=0;let published=0;
 const [err]=bindInput(():Result<int>=>{order=order*10+1;return ok(1);},(raw:int):Result<int>=>{order=order*10+2;return ok(raw+1);},(value:int)=>{order=order*10+3;published=value;});
 return err==nil&&order==123&&published==2;
}
export function Nullable(present:boolean):Result<User|null>{
 return decodeInput(():Result<boolean>=>{return ok(present);},(flag:boolean):Result<User|null>=>{if(!flag){return ok(null);}return ok(new User("present",[],null,0));});
}
export function MapValue(valid:boolean):Result<byte[]>{
 const value=decodeInput(():Result<Map<bstring,bstring>>=>{const raw=makeMap<bstring,bstring>();if(valid){raw[b"key"]=b"value";}else{raw[b"\xff"]=b"value";}return ok(raw);},(raw:Map<bstring,bstring>):Result<Map<string,string>>=>{
  const checked=makeMap<string,string>();for(const [key,value] of raw){const k=string(key)?;const v=string(value)?;checked[k]=v;}return ok(checked);
 })?;return jsonSnapshot(value);
}
function panicStage():int{const values:int[]=[];return values[0];}
export function FailureIdentity(source:error,readStage:boolean):error{
 const [err]=bindInput(():Result<int>=>{if(readStage){return fail(source);}return ok(1);},(value:int):Result<int>=>{return fail(source);},(value:int)=>{});
 return err;
}
export function Panic(stage:int):void{
 const [err]=bindInput(():Result<int>=>{if(stage==0){const _=panicStage();}return ok(1);},(value:int):Result<int>=>{if(stage==1){const _=panicStage();}return ok(value);},(value:int)=>{if(stage==2){const _=panicStage();}});
 if(err!=nil){const _=panicStage();}
}`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "inputadapters")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	mapped, diagnostics, err := EmitGoWithSourceMap([]string{path}, "inputadapters")
	if err != nil || len(diagnostics) != 0 || !bytes.Equal(generated, mapped.GoSource) {
		t.Fatalf("mapped: %v %v", err, diagnostics)
	}
	runGeneratedGoDifferentialTest(t, root, "input-adapters.test", generated, inputAdaptersReference, inputAdaptersComparison)
}

func TestInputAdaptersDoNotAuthorizeOpaqueWrites(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`import go json from "encoding/json";function decode<T>(payload:byte[],target:*T):error{return json.Unmarshal(payload,target);}`,
		`import {decodeInput} from "kinmokusei/input";import go json from "encoding/json";function decode<T>(payload:byte[],target:*T):Result<T>{return decodeInput(():Result<T>=>{const err=json.Unmarshal(payload,target);if(err!=nil){return fail(err);}return ok(*target);},(value:T):Result<T>=>{return ok(value);});}`,
		`import {decodeInput} from "kinmokusei/input";import go json from "encoding/json";class User{public name:string="safe";}function f():Result<User>{return decodeInput(():Result<User>=>{const user=new User();const err=json.Unmarshal(nil,user);if(err!=nil){return fail(err);}return ok(user);},(user:User):Result<User>=>{return ok(user);});}`,
	} {
		path := filepath.Join(t.TempDir(), "invalid.km")
		result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, diagnostic := range result.Diagnostics {
			if strings.Contains(diagnostic.Message, "cannot use") {
				found = true
			}
		}
		if !found {
			t.Fatalf("opaque write accepted: %v", result.Diagnostics)
		}
	}
}
