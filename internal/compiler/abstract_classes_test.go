package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAbstractClassesMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"go.mod": "module abstract-classes.test\n\ngo 1.23\n",
		"model.km": `import go errors from "errors";
export interface Reader<T>{function read():T;}
export abstract class Base<T> implements Reader<T>{
  constructor(protected value:T){}
  public abstract function read():T;
  protected abstract function transform(value:T):T;
  public function run():T{return this.transform(this.read());}
}
export abstract class Middle<U> extends Base<U>{
  constructor(value:U){super(value);}
  public override function read():U{return this.value;}
}
export class Leaf extends Middle<int>{
  constructor(value:int){super(value);}
  protected override function transform(value:int):int{return value+1;}
}
export abstract class Again extends Leaf{
  constructor(value:int){super(value);}
  public abstract override function read():int;
}
export final class Last extends Again{
  constructor(value:int){super(value);}
  public final override function read():int{return this.value*2;}
}
export abstract class Service implements error{
	public abstract function error():string;
  public abstract function load(...values:int[]):Result<int>;
  public abstract function touch():void;
}
export class Sum extends Service{
	public override function error():string{return "sum service";}
  public count:int=0;
  public override function load(...values:int[]):Result<int>{let sum=0;for(const value of values){if(value<0){return fail(errors.New("negative"));}sum+=value;}return ok(sum);}
  public override function touch():void{this.count++;}
}
abstract class Building{
  constructor(){this.invoke();}
  private function invoke():void{this.missing();}
  public abstract function missing():void;
}
class Built extends Building{public override function missing():void{}}
export function ConstructionFails():void{const value=new Built();}
`,
		"bridge.km": `export {Base as Repository,Reader,Leaf,Last,Service,Sum,ConstructionFails} from "./model";`,
		"entry.km": `import {Repository,Reader,Leaf,Last,Service,Sum,ConstructionFails} from "./bridge";
function inject(value:Repository<int>):int{const read=value.read;return value.run()+read();}
function read<T>(value:Reader<T>):T{return value.read();}
export function Run(n:int):int[]{
  const leaf=new Leaf(n);const last=new Last(n);
  const repo:Repository<int>=last;
  const required=repo as! Last;
  const [projected,matched]=repo as? Last;
  let same=0;if(matched && required==last && projected==last){same=1;}
  const values:Repository<int>[]=[leaf,last];
  return [inject(leaf),inject(last),read(repo),values[0].run(),same];
}
export function Loads(values:int[]):Result<int>{
  const sum=new Sum();const service:Service=sum;service.touch();
  const load=service.load;const result=load(values...)?;return ok(result+sum.count);
}
export function Build():void{ConstructionFails();}
`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	generated, diagnostics, err := EmitGo([]string{filepath.Join(root, "entry.km")}, "abstractclasses")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, name := range []string{"Base", "Middle", "Again", "Service"} {
		if strings.Contains(string(generated), "func New"+name+"(") || strings.Contains(string(generated), "func New"+name+"[") {
			t.Fatalf("abstract constructor emitted for %s", name)
		}
	}
	reference := `package reference
import "errors"
type reader interface{Read()int}
type repository interface{reader;Run()int}
type leaf struct{value int}
func(v *leaf)Read()int{return v.value}
func(v *leaf)Run()int{return v.Read()+1}
type last struct{value int}
func(v *last)Read()int{return v.value*2}
func(v *last)Run()int{return v.Read()+1}
func inject(v repository)int{read:=v.Read;return v.Run()+read()}
func Run(n int)[]int{a:=&leaf{n};b:=&last{n};var repo repository=b;required:=repo.(*last);projected,matched:=repo.(*last);same:=0;if matched&&required==b&&projected==b{same=1};values:=[]repository{a,b};return []int{inject(a),inject(b),repo.Read(),values[0].Run(),same}}
type service interface{Load(...int)(int,error);Touch()}
type sum struct{count int}
func(s *sum)Touch(){s.count++}
func(s *sum)Load(values ...int)(int,error){n:=0;for _,v:=range values{if v<0{return 0,errors.New("negative")};n+=v};return n,nil}
func Loads(values []int)(int,error){s:=&sum{};var v service=s;v.Touch();load:=v.Load;n,err:=load(values...);if err!=nil{return 0,err};return n+s.count,nil}
`
	comparison := `package abstractclasses_test
import("testing";"reflect";"fmt";"strings";g "abstract-classes.test";r "abstract-classes.test/reference")
func TestDI(t *testing.T){for _,n:=range []int{-9,0,1,20}{if !reflect.DeepEqual(g.Run(n),r.Run(n)){t.Fatal(n,g.Run(n),r.Run(n))}};for _,values:=range [][]int{nil,{}, {1,2,3},{1,-1,3}}{got,err:=g.Loads(values);want,werr:=r.Loads(values);if got!=want||fmt.Sprint(err)!=fmt.Sprint(werr){t.Fatal(got,err,want,werr)}}}
func TestGoAPI(t *testing.T){v:=g.NewLast(9);base:=g.UpcastLastToBase(v);read:=base.Read;var contract g.Reader[int]=base;if read()!=18||contract.Read()!=18||base.Run()!=19{t.Fatal("dispatch")};if got,ok:=g.DowncastBaseToLast(base);!ok||got!=v{t.Fatal("identity")};var service error=g.UpcastSumToService(g.NewSum());if service.Error()!="sum service"{t.Fatal(service)}}
func TestUnimplementedPhase(t *testing.T){for _,call:=range []func(){g.Build,func(){new(g.Base[int]).Read()}}{func(){defer func(){if problem:=recover();problem==nil||!strings.Contains(fmt.Sprint(problem),"abstract method"){t.Fatal(problem)}}();call()}()}}
`
	runGeneratedGoDifferentialTest(t, root, "abstract-classes.test", generated, reference, comparison)
}
