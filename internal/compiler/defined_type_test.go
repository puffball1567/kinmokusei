package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefinedTypesAndAliasesMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "defined_types.otm")
	if err := os.WriteFile(source, []byte(`
type UserID = distinct string;
type OrderID = distinct string;
type Score = distinct int;
type Tags = distinct string[];
type Scores = distinct Map<string, int>;
type Pair = distinct [2]int;
alias UserIDText = string;

function NewUserID(value: string): UserID { return UserID(value); }
function UserIDString(value: UserID): string { return string(value); }
function JoinUserID(left: UserID, right: UserID): UserID { return left + right; }
function AddScore(left: Score, right: Score): Score { return left + right; }
function ScoreLess(left: Score, right: Score): boolean { return left < right; }
function Lookup(values: Map<UserID, int>, key: UserID): int { return values[key]; }
function AliasText(value: UserIDText): string { return value; }
function TagsBehavior(values: string[]): int {
  let tags = Tags(values);
  tags = append(tags, "tail");
  tags[0] = "changed";
  return len(tags) * 100 + len(tags[0]);
}
function ScoresBehavior(values: Map<string, int>): int {
  const scores = Scores(values);
  scores["extra"] = 2;
  return scores["value"] * 10 + scores["extra"];
}
function PairBehavior(values: [2]int): int {
  let pair = Pair(values);
  pair[1] += 1;
  return pair[0] * 10 + pair[1];
}
function Present(value: UserID): Result<UserID> { return ok(value); }
function IdentityUserID(value: UserID): UserID { return identity(value); }
function identity<T>(value: T): T { return value; }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "definedtypes")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"type UserID string",
		"type OrderID string",
		"type Score int",
		"type Tags []string",
		"type Scores map[string]int",
		"type Pair [2]int",
		"type UserIDText = string",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

type UserID string
type OrderID string
type Score int
type Tags []string
type Scores map[string]int
type Pair [2]int
type UserIDText = string

func NewUserID(value string) UserID { return UserID(value) }
func UserIDString(value UserID) string { return string(value) }
func JoinUserID(left, right UserID) UserID { return left + right }
func AddScore(left, right Score) Score { return left + right }
func ScoreLess(left, right Score) bool { return left < right }
func Lookup(values map[UserID]int, key UserID) int { return values[key] }
func AliasText(value UserIDText) string { return value }
func TagsBehavior(values []string) int { tags := Tags(values); tags = append(tags, "tail"); tags[0] = "changed"; return len(tags)*100 + len(tags[0]) }
func ScoresBehavior(values map[string]int) int { scores := Scores(values); scores["extra"] = 2; return scores["value"]*10 + scores["extra"] }
func PairBehavior(values [2]int) int { pair := Pair(values); pair[1]++; return pair[0]*10 + pair[1] }
func Present(value UserID) (UserID, error) { return value, nil }
func IdentityUserID(value UserID) UserID { return identity(value) }
func identity[T any](value T) T { return value }
`
	testSource := `package definedtypes_test

import (
  "testing"
  generated "definedtypes.test"
  reference "definedtypes.test/reference"
)

func TestDefinedTypes(t *testing.T) {
  for _, value := range []string{"", "onsen", "温泉卵"} {
    gotID, wantID := generated.NewUserID(value), reference.NewUserID(value)
    if got, want := generated.UserIDString(gotID), reference.UserIDString(wantID); got != want { t.Errorf("UserIDString(%q) = %q, Go = %q", value, got, want) }
    if got, want := string(generated.JoinUserID(gotID, gotID)), string(reference.JoinUserID(wantID, wantID)); got != want { t.Errorf("JoinUserID(%q) = %q, Go = %q", value, got, want) }
    if got, want := string(generated.IdentityUserID(gotID)), string(reference.IdentityUserID(wantID)); got != want { t.Errorf("IdentityUserID(%q) = %q, Go = %q", value, got, want) }
    gotResult, gotErr := generated.Present(gotID)
    wantResult, wantErr := reference.Present(wantID)
    if string(gotResult) != string(wantResult) || (gotErr == nil) != (wantErr == nil) { t.Errorf("Present(%q) = (%q, %v), Go = (%q, %v)", value, gotResult, gotErr, wantResult, wantErr) }
    if got, want := generated.AliasText(value), reference.AliasText(value); got != want { t.Errorf("AliasText(%q) = %q, Go = %q", value, got, want) }
  }
  for _, values := range [][2]int{{0, 0}, {-1, 2}, {9, 3}} {
    gotLeft, gotRight := generated.Score(values[0]), generated.Score(values[1])
    wantLeft, wantRight := reference.Score(values[0]), reference.Score(values[1])
    if got, want := int(generated.AddScore(gotLeft, gotRight)), int(reference.AddScore(wantLeft, wantRight)); got != want { t.Errorf("AddScore(%v) = %d, Go = %d", values, got, want) }
    if got, want := generated.ScoreLess(gotLeft, gotRight), reference.ScoreLess(wantLeft, wantRight); got != want { t.Errorf("ScoreLess(%v) = %v, Go = %v", values, got, want) }
    if got, want := generated.PairBehavior(values), reference.PairBehavior(values); got != want { t.Errorf("PairBehavior(%v) = %d, Go = %d", values, got, want) }
  }
  gotKey, wantKey := generated.NewUserID("key"), reference.NewUserID("key")
  if got, want := generated.Lookup(map[generated.UserID]int{gotKey: 42}, gotKey), reference.Lookup(map[reference.UserID]int{wantKey: 42}, wantKey); got != want { t.Errorf("Lookup = %d, Go = %d", got, want) }
  gotTags, wantTags := []string{"head"}, []string{"head"}
  if got, want := generated.TagsBehavior(gotTags), reference.TagsBehavior(wantTags); got != want || gotTags[0] != wantTags[0] { t.Errorf("TagsBehavior = (%d, %q), Go = (%d, %q)", got, gotTags[0], want, wantTags[0]) }
  gotScores, wantScores := map[string]int{"value": 4}, map[string]int{"value": 4}
  if got, want := generated.ScoresBehavior(gotScores), reference.ScoresBehavior(wantScores); got != want || gotScores["extra"] != wantScores["extra"] { t.Errorf("ScoresBehavior = (%d, %d), Go = (%d, %d)", got, gotScores["extra"], want, wantScores["extra"]) }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "definedtypes.test", generated, referenceSource, testSource)
}

func TestLinkedDefinedTypeAndAliasMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	dependency := filepath.Join(temporary, "identifiers.otm")
	entry := filepath.Join(temporary, "entry.otm")
	if err := os.WriteFile(dependency, []byte(`type UserID = distinct string; alias UserIDText = string; function makeID(value: string): UserID { return UserID(value); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`import { UserID, UserIDText, makeID } from "./identifiers"; function linked(value: UserIDText): string { const id: UserID = makeID(value); return string(id); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "definedtypeslinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	referenceSource := `package reference
type UserID string
type UserIDText = string
func makeID(value string) UserID { return UserID(value) }
func Linked(value UserIDText) string { id := makeID(value); return string(id) }
`
	testSource := `package definedtypeslinked
import (
  "testing"
  reference "definedtypes-linked.test/reference"
)
func TestLinked(t *testing.T) {
  for _, value := range []string{"", "onsen", "温泉卵"} {
    if got, want := linked(value), reference.Linked(value); got != want { t.Errorf("linked(%q) = %q, Go = %q", value, got, want) }
  }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "definedtypes-linked.test", generated, referenceSource, testSource)
}
