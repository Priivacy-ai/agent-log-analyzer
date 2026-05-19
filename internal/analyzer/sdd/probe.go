package sdd

import (
	"bytes"
	"context"
	"io"
	"os/exec"
)

// maxVersionOutputBytes bounds how much stdout we read from a probed binary
// before we treat it as runaway output. 4 KiB is generous for `--version`
// strings and tight enough to prevent a hostile binary from ballooning memory.
const maxVersionOutputBytes = 4 * 1024

// CLIProbe abstracts the two operations the evaluator needs to perform on
// CLI tooling: a non-leaky presence check and a sandboxed version capture.
// Implementations MUST preserve the privacy invariants described in
// contracts/cli-probe.md — in particular, the resolved executable path
// returned by exec.LookPath MUST NOT escape an implementation.
type CLIProbe interface {
	// LookPath reports whether the binary is reachable on PATH. The resolved
	// path is intentionally discarded and not exposed through any return
	// value, error, or receiver field.
	LookPath(name string) (found bool)

	// Version executes the binary with the supplied args under a sanitized
	// environment (empty env, nil stdin, no shell) bounded by ctx. The
	// returned rawOutput is consumed only by in-package normalizers; callers
	// MUST NOT propagate it to reports, logs, or external sinks.
	Version(ctx context.Context, name string, args []string) (rawOutput string, ok bool)
}

// RealProbe is the production CLIProbe. The zero value is usable; prefer
// returning it via NewRealProbe so callers see the CLIProbe interface and
// cannot accidentally reach for a path-leaking field that does not exist.
type RealProbe struct{}

// NewRealProbe returns a production CLIProbe. The CLIProbe return type is
// deliberate: keeping the concrete RealProbe out of caller code makes it
// harder for future contributors to add a path-leaking method.
func NewRealProbe() CLIProbe { return RealProbe{} }

// LookPath returns only a boolean. The resolved path from exec.LookPath is
// deliberately discarded via the blank identifier and is never captured.
func (RealProbe) LookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Version invokes the binary with args under a sealed environment:
//   - empty Env (no inherited variables that could be observed by the child)
//   - nil Stdin (the child cannot read from the parent process)
//   - no shell interpolation (args are passed verbatim to exec)
//   - context-bounded execution (callers MUST set a deadline; NFR-002 caps it at 2s)
//
// stdout is captured into a 4 KiB-bounded buffer; stderr is discarded by
// exec.Cmd.Output. On any error, non-zero exit, or timeout the function
// returns ("", false). The returned rawOutput MUST be normalized by the
// caller (see NormalizeVersionBucket) before it is stored or emitted.
func (RealProbe) Version(ctx context.Context, name string, args []string) (string, bool) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = []string{}
	cmd.Stdin = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", false
	}
	// Discard stderr by leaving cmd.Stderr nil — exec routes it to /dev/null
	// for us; we deliberately do NOT capture it to avoid leaking environment
	// info into the bucket pipeline.
	if err := cmd.Start(); err != nil {
		return "", false
	}

	var buf bytes.Buffer
	// io.LimitReader caps the bytes we read; anything beyond the cap is
	// discarded. We still wait on the process so it doesn't become a zombie.
	_, copyErr := io.Copy(&buf, io.LimitReader(stdout, maxVersionOutputBytes))
	// Drain any remainder so the child can exit cleanly without blocking on
	// a full pipe; we discard the surplus.
	_, _ = io.Copy(io.Discard, stdout)

	waitErr := cmd.Wait()
	if copyErr != nil || waitErr != nil {
		return "", false
	}
	return buf.String(), true
}

// FakeProbe is the test fake every unit test in the sdd package uses. The
// real exec codepath is exercised only by RealProbe's integration tests.
type FakeProbe struct {
	Installed map[string]bool
	Versions  map[string]string // rawOutput per binary; "" means probe fails
}

// LookPath reports whether the binary was registered as installed in the
// fake. It never touches the filesystem.
func (f FakeProbe) LookPath(name string) bool {
	return f.Installed[name]
}

// Version returns the canned raw output for name. An empty string is
// treated as "probe failed" (ok=false) to mirror RealProbe's failure mode.
func (f FakeProbe) Version(_ context.Context, name string, _ []string) (string, bool) {
	v, ok := f.Versions[name]
	if !ok || v == "" {
		return "", false
	}
	return v, true
}
