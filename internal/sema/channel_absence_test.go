package sema

import (
	"strings"
	"testing"
)

func TestChannelAbsenceAndSelectPresence(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"nullable receive":          `class C{public n:int=1;}function f(ch:GoChannel<C>):int{const v=<-ch;if(v===null){return 0;}return v.n;}`,
		"fallback":                  `class C{public n:int=1;}function f(ch:GoReceiveChannel<C>):int{return ((<-ch)??new C()).n;}`,
		"assertion":                 `class C{public n:int=1;}function f(ch:GoChannel<C>):int{return (<-ch)!.n;}`,
		"safe scalar":               `function f(ch:GoChannel<int>):int{return <-ch;}`,
		"nullable payload":          `class C{}function f(ch:GoChannel<C|null>):C|null{return <-ch;}`,
		"safe zero container":       `function f(ch:GoChannel<int[]>):int[]{return <-ch;}`,
		"discard statement":         `class C{}struct R{public v:C;}function f<T>(ch:GoChannel<T>,rows:GoChannel<R>):void{<-ch;<-rows;}`,
		"blank discard":             `class C{}struct R{public v:C;}function f<T>(ch:GoChannel<T>,rows:GoChannel<R>):void{const _=<-ch;_=<-rows;}`,
		"generic checked":           `function f<T>(ch:GoChannel<T>,fallback:T):T{const [v,open]=<-ch;if(!open){return fallback;}return v;}`,
		"nilable bound":             `constraint P=~*int;function f<T extends P>(ch:GoChannel<T>,fallback:T):T{return (<-ch)??fallback;}`,
		"generic directional":       `class C{}constraint Ch=~GoChannel<C>|~GoReceiveChannel<C>;function f<T extends Ch>(ch:T):C|null{return <-ch;}`,
		"select one":                `class C{public n:int=1;}function f(ch:GoChannel<C>):int{select{case const v=<-ch{if(v===null){return 0;}return v.n;}}}`,
		"select checked":            `class C{public n:int=1;}function f(ch:GoChannel<C>):int{select{case const [v,open]=<-ch{if(!open){return 0;}return v.n;}}}`,
		"select record":             `class C{}struct R{public v:C;}function f(ch:GoChannel<R>,fallback:R):R{select{case const [v,open]=<-ch{if(open){return v;}return fallback;}}}`,
		"select generic":            `function f<T>(ch:GoChannel<T>,fallback:T):T{select{case const [v,open]=<-ch{if(!open){return fallback;}return v;}}}`,
		"select assignment":         `class C{public n:int=1;}function f(ch:GoChannel<C>):int{let v=new C();let open=false;select{case [v,open]=<-ch{if(!open){return 0;}return v.n;}}}`,
		"selected proof after exit": `class C{public n:int=1;}function f(ch:GoChannel<C>):int{let v=new C();let open=false;select{case [v,open]=<-ch{if(!open){return 0;}}}return v.n;}`,
		"select discard":            `function f<T>(ch:GoChannel<T>):void{select{case <-ch{}}select{case const _=<-ch{}}select{case _=<-ch{}}}`,
		"nullable interface target": `interface I{function read():int;}class C implements I{public function read():int{return 1;}}function f(ch:GoChannel<C>):I|null{let v:I|null=null;select{case v=<-ch{}}return v;}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("rejected: %v", diagnostics)
			}
		})
	}
}

func TestRejectsUncheckedChannelAbsence(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ input, want string }{
		"member read":                     {`class C{public n:int=1;}function f(ch:GoChannel<C>):int{return (<-ch).n;}`, "nullable"},
		"non-null return":                 {`class C{}function f(ch:GoChannel<C>):C{return <-ch;}`, "cannot use"},
		"unknown generic":                 {`function f<T>(ch:GoChannel<T>):T{return <-ch;}`, "no safe zero"},
		"unsafe record":                   {`class C{}struct R{public v:C;}function f(ch:GoChannel<R>):R{return <-ch;}`, "no safe zero"},
		"select member":                   {`class C{public n:int=1;}function f(ch:GoChannel<C>):int{select{case const v=<-ch{return v.n;}}}`, "nullable"},
		"select unchecked pair":           {`class C{public n:int=1;}function f(ch:GoChannel<C>):int{select{case const [v,open]=<-ch{return v.n;}}}`, "may be absent"},
		"select record one":               {`class C{}struct R{public v:C;}function f(ch:GoChannel<R>):R{select{case const v=<-ch{return v;}}}`, "no safe zero"},
		"select reset flag":               {`class C{public n:int=1;}function f(ch:GoChannel<C>):int{select{case let [v,open]=<-ch{open=true;if(open){return v.n;}return 0;}}}`, "may be absent"},
		"select global":                   {`class C{}let v=new C();function f(ch:GoChannel<C>):void{let open=false;select{case [v,open]=<-ch{}}}`, "local payload"},
		"select assignment outside proof": {`class C{public n:int=1;}function f(ch:GoChannel<C>):int{let v=new C();let open=false;select{case [v,open]=<-ch{}}return v.n;}`, "may be absent"},
		"select nullable still nullable":  {`class C{public n:int=1;}function f(ch:GoChannel<C|null>):int{select{case const [v,open]=<-ch{if(open){return v.n;}return 0;}}}`, "nullable"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			diagnostics := strings.Join(checkSource(t, test.input), "\n")
			if !strings.Contains(diagnostics, test.want) {
				t.Fatalf("diagnostics=%q want=%q", diagnostics, test.want)
			}
		})
	}
}
