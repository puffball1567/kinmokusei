package compiler

// The reference uses ordinary Go staging, independent of the source module.
const inputAdaptersReference = "package reference\n" + `
import("bytes";"encoding/json";"errors";"io";"net/url";"unicode/utf8")
type RawChild struct{Title string ` + "`json:\"title\"`" + `}
type RawInput struct{Name string ` + "`json:\"name\"`" + `;Children []RawChild ` + "`json:\"children\"`" + `;Optional *RawChild ` + "`json:\"optional\"`" + `;Count int ` + "`json:\"count\"`" + `}
type Child struct{Title string ` + "`json:\"title\"`" + `}
type User struct{Name string ` + "`json:\"name\"`" + `;Children []*Child ` + "`json:\"children\"`" + `;Optional *Child ` + "`json:\"optional\"`" + `;Count int ` + "`json:\"count\"`" + `}
type Record struct{Name string ` + "`json:\"name\"`" + `;Titles []string ` + "`json:\"titles\"`" + `}
func fresh()RawInput{return RawInput{Children:[]RawChild{}}}
func read(payload []byte,strict bool)(RawInput,error){
 raw:=fresh();var err error
 if strict{decoder:=json.NewDecoder(bytes.NewReader(payload));decoder.DisallowUnknownFields();err=decoder.Decode(&raw);if err==nil{var extra string;if decoder.Decode(&extra)!=io.EOF{err=errors.New("trailing JSON value")}}}else{err=json.Unmarshal(payload,&raw)}
 return raw,err
}
func construct(raw RawInput)(*User,error){
 if !utf8.ValidString(raw.Name)||raw.Name==""||raw.Count<0{return nil,errors.New("invalid input")}
 children:=[]*Child{};for _,child:=range raw.Children{if !utf8.ValidString(child.Title){return nil,errors.New("invalid UTF-8")};children=append(children,&Child{child.Title})}
 var optional *Child;if raw.Optional!=nil{if !utf8.ValidString(raw.Optional.Title){return nil,errors.New("invalid UTF-8")};optional=&Child{raw.Optional.Title}}
 return &User{raw.Name,children,optional,raw.Count},nil
}
func snapshot(value *User)string{encoded,err:=json.Marshal(value);if err!=nil{return "snapshot failed"};return string(encoded)}
func apply(readInput func()(RawInput,error))(string,string,int,bool){
 trace:=1;target:=&User{"original",[]*Child{{"kept"}},nil,9};alias:=target
 raw,err:=readInput();if err==nil{trace=12;var candidate *User;candidate,err=construct(raw);if err==nil{target=candidate}}
 return snapshot(target),snapshot(alias),trace,err==nil
}
func Apply(payload []byte,strict bool)(string,string,int,bool){return apply(func()(RawInput,error){return read(payload,strict)})}
func Query(query string)(string,string,int,bool){return apply(func()(RawInput,error){values,err:=url.ParseQuery(query);raw:=fresh();raw.Name=values.Get("name");return raw,err})}
func StructValue(payload []byte)(string,error){raw,err:=read(payload,false);if err!=nil{return "",err};if !utf8.ValidString(raw.Name){return "",errors.New("invalid UTF-8")};titles:=[]string{};for _,child:=range raw.Children{if !utf8.ValidString(child.Title){return "",errors.New("invalid UTF-8")};titles=append(titles,child.Title)};encoded,err:=json.Marshal(Record{raw.Name,titles});return string(encoded),err}
func Nullable(present bool)(*User,error){if !present{return nil,nil};return &User{"present",[]*Child{},nil,0},nil}
func MapValue(valid bool)([]byte,error){if !valid{return nil,errors.New("invalid UTF-8")};return json.Marshal(map[string]string{"key":"value"})}
func panicStage()int{values:=[]int{};return values[0]}
func FailureIdentity(source error,readStage bool)error{return source}
func Panic(stage int){switch stage{case 0:_=panicStage();case 1:_=panicStage();case 2:_=panicStage()}}
`

const inputAdaptersComparison = `package inputadapters_test
import("bytes";"errors";"testing";g "input-adapters.test";r "input-adapters.test/reference")
func TestJSON(t *testing.T){
 for _,payload:=range []string{
  "{}","null","{broken",` + "`" + `{"name":"Alice","children":[{"title":"雪"}],"optional":{"title":"present"},"count":2}` + "`" + `,
  ` + "`" + `{"name":"Alice","children":null,"optional":null}` + "`" + `,
  ` + "`" + `{"name":"partly written","count":"wrong type"}` + "`" + `,
  ` + "`" + `{"name":"invalid count","count":-1}` + "`" + `,
  ` + "`" + `{"name":"Alice","unknown":1}` + "`" + `,
  ` + "`" + `{"name":"Alice"} {"another":1}` + "`" + `,
  ` + "`" + `{"name":"Alice"} garbage` + "`" + `,
  "{\"name\":\"\xff\"}",
 }{for _,strict:=range []bool{false,true}{
  a,b,c,d:=g.Apply([]byte(payload),strict);e,f,h,i:=r.Apply([]byte(payload),strict)
  if a!=e||b!=f||c!=h||d!=i{t.Fatalf("JSON %q strict=%v: %s / %s / %d / %v vs %s / %s / %d / %v",payload,strict,a,b,c,d,e,f,h,i)}
  x,xe:=g.StructValue([]byte(payload));y,ye:=r.StructValue([]byte(payload));if x!=y||(xe==nil)!=(ye==nil){t.Fatalf("struct %q: %q/%v vs %q/%v",payload,x,xe,y,ye)}
 }}
}
func TestQuery(t *testing.T){for _,query:=range []string{"name=Alice","name=%E9%9B%AA","name=%ff","name=","name=%ZZ"}{a,b,c,d:=g.Query(query);e,f,h,i:=r.Query(query);if a!=e||b!=f||c!=h||d!=i{t.Fatalf("query %q: %s/%s/%d/%v vs %s/%s/%d/%v",query,a,b,c,d,e,f,h,i)}}}
func TestFailures(t *testing.T){if !g.CustomFailure(false)||!g.CustomFailure(true)||!g.InvalidRaw()||!g.PublishOrder(){t.Fatal("failed stage published a value, invoked construction, or changed the destination")};source:=errors.New("original");for _,readStage:=range []bool{false,true}{a,b:=g.FailureIdentity(source,readStage),r.FailureIdentity(source,readStage);if a!=b||a!=source{t.Fatal("error identity changed")}}}
func TestNullableAndMaps(t *testing.T){for _,present:=range []bool{false,true}{a,ae:=g.Nullable(present);b,be:=r.Nullable(present);if (a==nil)!=(b==nil)||(ae==nil)!=(be==nil){t.Fatal("nullable construction")};if a!=nil&&b!=nil&&a.Name!=b.Name{t.Fatal("nullable value")};x,xe:=g.MapValue(present);y,ye:=r.MapValue(present);if !bytes.Equal(x,y)||(xe==nil)!=(ye==nil){t.Fatal("map contract validation")}}}
func caught(call func())(value any){defer func(){value=recover()}();call();return nil}
func TestPanics(t *testing.T){for stage:=0;stage<3;stage++{a:=caught(func(){g.Panic(stage)});b:=caught(func(){r.Panic(stage)});if a!=b||a==nil{t.Fatalf("stage %d: panic %v vs %v",stage,a,b)}}}
`
