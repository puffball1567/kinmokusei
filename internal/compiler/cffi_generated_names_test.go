package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestIncomingCFFINeighboringHelperNames(t *testing.T) {
	t.Parallel()
	requireCCompiler(t)
	root := t.TempDir()
	artifacts, err := GenerateCFFI([]byte(`{
"schemaVersion":1,"package":"fixture","header":"fixture.h","threadPolicy":"threadSafe",
"structs":[{"name":"Point","cType":"fixture_point","fields":[{"name":"X","cName":"x","type":"int32"}]}],
"callbacks":[{"name":"Data","lifetime":"registered","parameters":[],"result":"void"},{"name":"Data_release","lifetime":"callScoped","parameters":[],"result":"void"},{"name":"FromPoint","lifetime":"callScoped","parameters":[],"result":"void"}],
"callbackRegistrations":[{"name":"Watch","callback":"Data","register":"fixture_register","unregister":"fixture_unregister"}],
"functions":[{"name":"Run","symbol":"fixture_run","parameters":[{"name":"callback","type":"Data_release"}],"result":"void","convention":"direct"},{"name":"RunAgain","symbol":"fixture_run","parameters":[{"name":"callback","type":"Data_release"}],"result":"void","convention":"direct"},{"name":"Fire","symbol":"fixture_fire","parameters":[],"result":"void","convention":"direct"},{"name":"Echo","symbol":"fixture_echo","parameters":[{"name":"point","type":"Point"}],"result":"Point","convention":"direct"}]
}`))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":     "module fixture.test\n\ngo 1.23\n",
		"binding.go": string(artifacts.Source),
		"fixture.h": `#include <stdint.h>
#include <stddef.h>
typedef struct { int32_t x; } fixture_point;
int32_t fixture_register(void (*callback)(void *), void *context);
int32_t fixture_unregister(void (*callback)(void *), void *context);
void fixture_run(void (*callback)(void *), void *context);
void fixture_fire(void);
fixture_point fixture_echo(fixture_point point);
`,
		"fixture.c": `#include "fixture.h"
static void (*registered)(void *);
static void *registered_context;
int32_t fixture_register(void (*callback)(void *), void *context){registered=callback;registered_context=context;return 0;}
int32_t fixture_unregister(void (*callback)(void *), void *context){if(registered!=callback||registered_context!=context){return 1;}registered=NULL;registered_context=NULL;return 0;}
void fixture_run(void (*callback)(void *), void *context){callback(context);}
void fixture_fire(void){if(registered!=NULL){registered(registered_context);}}
fixture_point fixture_echo(fixture_point point){return point;}
`,
		"cmd/main.go": `package main
import f "fixture.test"
func main(){
 calls:=0
 watch,err:=f.RegisterWatch(func(){calls++});if err!=nil{panic(err)}
 f.Fire()
 if err=f.Run(func(){calls+=10});err!=nil{panic(err)}
 if err=f.RunAgain(func(){calls+=100});err!=nil{panic(err)}
 if point:=f.Echo(f.Point{X:7});point.X!=7{panic("POD conversion failed")}
 if err=watch.Close();err!=nil{panic(err)}
 f.Fire()
 if calls!=111{panic("callback counts differ")}
}
`,
	}
	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	arguments := []string{"run", "-buildvcs=false", "./cmd"}
	if os.Getenv("KINMOKUSEI_DIFFERENTIAL_RACE") == "1" {
		arguments = []string{"run", "-race", "-buildvcs=false", "./cmd"}
	}
	command := exec.CommandContext(ctx, "go", arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("valid neighboring helper names failed to build/run: %v\n%s", err, output)
	}
	command = exec.CommandContext(ctx, "go", "vet", "./...")
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("valid neighboring helper names failed vet: %v\n%s", err, output)
	}
}
