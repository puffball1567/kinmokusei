package compiler

import (
	"flag"
	"os"
	"strconv"
	"testing"
)

func TestMain(m *testing.M) {
	flag.Parse()
	limitDefaultCompilerParallelism(flag.CommandLine)
	os.Exit(m.Run())
}

// Each differential test starts its own Go toolchain processes. Keep the
// default modest, without raising a smaller GOMAXPROCS-derived default or
// overriding an explicit go test -parallel setting.
func limitDefaultCompilerParallelism(flags *flag.FlagSet) {
	explicit := false
	flags.Visit(func(option *flag.Flag) {
		if option.Name == "test.parallel" {
			explicit = true
		}
	})
	if explicit {
		return
	}
	option := flags.Lookup("test.parallel")
	if option == nil {
		return
	}
	if value, err := strconv.Atoi(option.Value.String()); err == nil && value > 4 {
		_ = flags.Set("test.parallel", "4")
	}
}

func TestDefaultCompilerParallelism(t *testing.T) {
	for _, test := range []struct {
		name      string
		initial   int
		arguments []string
		want      int
	}{
		{"single CPU", 1, nil, 1},
		{"small machine", 2, nil, 2},
		{"at limit", 4, nil, 4},
		{"large machine", 32, nil, 4},
		{"explicit serial", 32, []string{"-test.parallel=1"}, 1},
		{"explicit higher", 32, []string{"-test.parallel", "8"}, 8},
	} {
		t.Run(test.name, func(t *testing.T) {
			flags := flag.NewFlagSet(test.name, flag.ContinueOnError)
			parallel := flags.Int("test.parallel", test.initial, "")
			if err := flags.Parse(test.arguments); err != nil {
				t.Fatal(err)
			}
			limitDefaultCompilerParallelism(flags)
			if *parallel != test.want {
				t.Fatalf("parallel = %d, want %d", *parallel, test.want)
			}
		})
	}
}
