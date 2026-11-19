package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestConstrainedCallbackCallSignature(t *testing.T) {
	t.Parallel()
	input := `alias Callback<E>=(v:E, count:int)=>E;
constraint C<E>=~Callback<E>;
function apply<E,F extends C<E>>(fn:F,value:E):E{return fn(value, 2);}`
	path := filepath.Join(t.TempDir(), "callback.km")
	label, active, parameters := signatureResult(t, signatureHelpAt(t, path, input, positionOf(input, "2);", 0)))
	if !strings.Contains(label, "fn(") || !strings.HasSuffix(label, "): E") || !strings.Contains(label, "int") || active != float64(1) || len(parameters) != 2 {
		t.Fatalf("signature=%q active=%v parameters=%v", label, active, parameters)
	}
}
