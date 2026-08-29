package token

import "testing"

func TestLookupIdentifierDistinguishesKeywordsAndNearMisses(t *testing.T) {
	for text, want := range keywords {
		if got := LookupIdentifier(text); got != want {
			t.Errorf("LookupIdentifier(%q) = %s, want %s", text, got, want)
		}
	}
	for _, text := range []string{"", "Function", "function1", "interfaces", "implements_", "日本語"} {
		if got := LookupIdentifier(text); got != Identifier {
			t.Errorf("LookupIdentifier(%q) = %s, want identifier", text, got)
		}
	}
}
