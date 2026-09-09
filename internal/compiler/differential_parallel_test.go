package compiler

import (
	"reflect"
	"testing"
)

func TestDifferentialBuildParallelism(t *testing.T) {
	for _, test := range []struct {
		name      string
		arguments []string
		want      []string
	}{
		{"default", []string{"test", "./..."}, []string{"test", "-p=2", "./..."}},
		{"race and tags", []string{"test", "-race", "-tags=fixture", "./..."}, []string{"test", "-p=2", "-race", "-tags=fixture", "./..."}},
		{"explicit equals", []string{"test", "-p=1", "./..."}, []string{"test", "-p=1", "./..."}},
		{"explicit separate", []string{"test", "-p", "3", "./..."}, []string{"test", "-p", "3", "./..."}},
		{"explicit long equals", []string{"test", "--p=1", "./..."}, []string{"test", "--p=1", "./..."}},
		{"explicit long separate", []string{"test", "--p", "3", "./..."}, []string{"test", "--p", "3", "./..."}},
		{"test parallel is separate", []string{"test", "-parallel=1", "./..."}, []string{"test", "-p=2", "-parallel=1", "./..."}},
		{"other command", []string{"build", "./..."}, []string{"build", "./..."}},
		{"empty", nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := append([]string(nil), test.arguments...)
			if got := limitDifferentialBuildParallelism(test.arguments); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("arguments = %v, want %v", got, test.want)
			}
			if !reflect.DeepEqual(test.arguments, original) {
				t.Fatal("modified the caller's arguments")
			}
		})
	}
}
