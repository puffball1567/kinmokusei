package attribution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Inspect runs only offline/read-only Go queries with the same environment and
// flags as the build. Node and an installed Kinmokusei source checkout are not
// needed by an installed keika binary.
func Inspect(directory string, environment, flags []string, sources []SourcePackage, tags []string) (Bundle, error) {
	run := func(arguments ...string) ([]byte, error) {
		command := exec.Command("go", arguments...)
		command.Dir, command.Env = directory, environment
		var stderr bytes.Buffer
		command.Stderr = &stderr
		data, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("cannot collect build attribution: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return data, nil
	}
	data, err := run("env", "-json", "GOROOT", "GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED")
	if err != nil {
		return Bundle{}, err
	}
	var toolchain Toolchain
	if err := json.Unmarshal(data, &toolchain); err != nil {
		return Bundle{}, err
	}
	arguments := append([]string{"list"}, flags...)
	arguments = append(arguments, "-mod=readonly", "-buildvcs=false", "-deps", "-json", ".")
	data, err = run(arguments...)
	if err != nil {
		return Bundle{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var packages []Package
	for {
		var pkg Package
		if err := decoder.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return Bundle{}, err
		}
		packages = append(packages, pkg)
	}
	return Collect(toolchain, packages, sources, tags)
}
