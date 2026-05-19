package sdd_test

// Structural bounded-shape test for NFR-003.
//
// Asserts that analyzer.EcosystemFingerprint has exactly the seven primitive
// fields documented in data-model.md, with the expected reflect.Kind for
// each, and that no field is a map (which would allow arbitrary user-keyed
// strings to escape into the serialized report). Also asserts that
// Ecosystem.WorkflowFingerprints is a slice whose element type is exactly
// EcosystemFingerprint.
//
// This file lives in package sdd_test (a separate compilation unit from
// package sdd) so it can import the parent analyzer package without
// creating an import cycle. The analyzer package imports sdd; sdd does
// not import analyzer; sdd_test (compiled only for `go test`) is free to
// reach back into analyzer.

import (
	"reflect"
	"testing"

	"github.com/robertdouglass/claude-log-analyzer/internal/analyzer"
)

func TestEcosystemFingerprintBoundedShape(t *testing.T) {
	expectFields := map[string]reflect.Kind{
		"ID":            reflect.String,
		"Confidence":    reflect.String,
		"Sources":       reflect.Slice,
		"EvidenceCount": reflect.Int,
		"Active":        reflect.Bool,
		"Installed":     reflect.Bool,
		"VersionBucket": reflect.String,
	}

	typ := reflect.TypeOf(analyzer.EcosystemFingerprint{})
	if got := typ.NumField(); got != len(expectFields) {
		t.Fatalf(
			"EcosystemFingerprint has %d fields; expected exactly %d. "+
				"If you added a field, ALSO update TestReportSerializationContainsNoForbiddenStrings "+
				"in internal/analyzer/leak_test.go to assert that the new field cannot carry any "+
				"of the forbidden-string canaries (NFR-001).",
			got, len(expectFields))
	}

	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		want, ok := expectFields[f.Name]
		if !ok {
			t.Errorf("unexpected field %q on EcosystemFingerprint; bounded-shape rule (NFR-003) forbids adding fields without updating the canary leak test", f.Name)
			continue
		}
		if f.Type.Kind() != want {
			t.Errorf("field %q kind = %s; expected %s", f.Name, f.Type.Kind(), want)
		}
		if f.Type.Kind() == reflect.Map {
			t.Errorf("field %q is a map; bounded-cardinality rule forbids maps keyed by user-derived strings on report-emitted records", f.Name)
		}
		if f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() != reflect.String {
			t.Errorf("field %q slice element kind = %s; expected string", f.Name, f.Type.Elem().Kind())
		}
	}

	// Also walk Ecosystem.WorkflowFingerprints and assert its element type
	// is exactly EcosystemFingerprint — so anyone swapping in a richer
	// element type (e.g., one with a free-text "evidence" field) is forced
	// to update this test.
	ecoTyp := reflect.TypeOf(analyzer.Ecosystem{})
	wfField, ok := ecoTyp.FieldByName("WorkflowFingerprints")
	if !ok {
		t.Fatal("Ecosystem.WorkflowFingerprints field missing; data-model.md requires it")
	}
	if wfField.Type.Kind() != reflect.Slice {
		t.Fatalf("WorkflowFingerprints kind = %s; expected slice", wfField.Type.Kind())
	}
	if wfField.Type.Elem() != typ {
		t.Fatalf("WorkflowFingerprints element type = %v; expected %v (the same EcosystemFingerprint struct). If you swapped the element type, update both this test and the canary leak test.", wfField.Type.Elem(), typ)
	}
}
