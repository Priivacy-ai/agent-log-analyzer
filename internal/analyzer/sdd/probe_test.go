package sdd

import (
	"context"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFakeProbe_BasicBehavior(t *testing.T) {
	fp := FakeProbe{
		Installed: map[string]bool{"openspec": true, "absent": false},
		Versions:  map[string]string{"openspec": "openspec 1.2.3", "blank": ""},
	}

	if !fp.LookPath("openspec") {
		t.Fatal("expected LookPath(openspec) = true")
	}
	if fp.LookPath("absent") {
		t.Fatal("expected LookPath(absent) = false")
	}
	if fp.LookPath("unknown") {
		t.Fatal("expected LookPath(unknown) = false (zero value)")
	}

	out, ok := fp.Version(context.Background(), "openspec", []string{"--version"})
	if !ok {
		t.Fatal("expected Version(openspec) ok=true")
	}
	if out != "openspec 1.2.3" {
		t.Fatalf("Version(openspec) = %q, want %q", out, "openspec 1.2.3")
	}

	if _, ok := fp.Version(context.Background(), "blank", nil); ok {
		t.Fatal("blank Versions entry should report ok=false")
	}
	if _, ok := fp.Version(context.Background(), "missing", nil); ok {
		t.Fatal("missing Versions entry should report ok=false")
	}
}

// TestRealProbe_LookPathReturnsBool confirms the signature contract: the
// only thing crossing the function boundary is a bool. There is no path,
// no error, no string anywhere on the return surface.
func TestRealProbe_LookPathReturnsBool(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go on PATH")
	}

	probe := RealProbe{}
	got := probe.LookPath("go")
	if !got {
		t.Fatal("expected RealProbe.LookPath(go) = true")
	}

	// Structural check: the LookPath method must return exactly one value,
	// and that value must be of kind bool. A future contributor cannot
	// add a path-leaking string return without this test breaking.
	mt, ok := reflect.TypeOf(probe).MethodByName("LookPath")
	if !ok {
		t.Fatal("RealProbe.LookPath method not found via reflection")
	}
	if mt.Type.NumOut() != 1 {
		t.Fatalf("LookPath must have exactly 1 return value, got %d", mt.Type.NumOut())
	}
	if mt.Type.Out(0).Kind() != reflect.Bool {
		t.Fatalf("LookPath must return bool, got %s", mt.Type.Out(0).Kind())
	}

	// Belt-and-suspenders: the fmt.Sprintf of the return value must never
	// contain a path separator (would indicate a resolved exec path leaked
	// through the boolean conversion somehow, e.g. via a custom type).
	rendered := fmt.Sprintf("%v", got)
	if strings.ContainsAny(rendered, "/\\") {
		t.Fatalf("LookPath return %q contains a path separator", rendered)
	}
}

// TestRealProbe_VersionHonorsContextDeadline asserts the call returns
// promptly under a tight deadline rather than hanging. Both ok=true (lucky
// fast OS) and ok=false (deadline hit before exec completed) are valid; the
// invariant is "no hang" and "if ok then rawOutput is non-empty".
func TestRealProbe_VersionHonorsContextDeadline(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go on PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	var (
		raw string
		ok  bool
	)
	go func() {
		raw, ok = RealProbe{}.Version(ctx, "go", []string{"version"})
		close(done)
	}()

	select {
	case <-done:
		// Acceptable outcomes: timeout (ok=false, raw=="") or a lucky
		// fast exec that beat the 5ms deadline (ok=true, raw!="").
		if ok && raw == "" {
			t.Fatal("ok=true must imply rawOutput != \"\"")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RealProbe.Version hung past the 2s NFR-002 ceiling")
	}
}

// TestRealProbe_VersionEmptyEnvNoStdin is the ONE test that touches the raw
// output. It verifies the real exec path works end-to-end against a known
// binary (`go version`) with empty Env, nil Stdin, and no shell. The raw
// output is asserted once and then deliberately discarded to model the
// caller contract: nothing outside the sdd package ever sees rawOutput.
func TestRealProbe_VersionEmptyEnvNoStdin(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go on PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	raw, ok := RealProbe{}.Version(ctx, "go", []string{"version"})
	if !ok {
		t.Fatal("expected ok=true probing `go version`")
	}
	if !strings.Contains(raw, "go") {
		t.Fatalf("rawOutput %q did not contain expected token 'go'", raw)
	}
	// Sanity: the bucket the caller would store is low-cardinality and
	// drops everything else from the raw output.
	bucket := NormalizeVersionBucket(raw)
	if bucket == "" {
		t.Fatalf("expected non-empty bucket from `go version` output, raw=%q", raw)
	}
	if bucket == raw {
		t.Fatal("bucket must never equal the raw input verbatim")
	}
	// Contract: discard raw after extracting the bucket. Reassign to a
	// throwaway to make the intent visible to future readers.
	_ = raw
}

// TestNewRealProbe_ReturnsCLIProbe documents that the constructor returns
// the interface type, not the concrete struct. This keeps callers from
// reaching for hypothetical path-leaking fields.
func TestNewRealProbe_ReturnsCLIProbe(t *testing.T) {
	var _ CLIProbe = NewRealProbe()
}
