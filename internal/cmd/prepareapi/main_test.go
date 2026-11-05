package main

import (
	"bytes"
	"os"
	"testing"
)

func snapshotFiles(t *testing.T) ([]byte, []byte) {
	t.Helper()
	snapshot, err := os.ReadFile("../../../api/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile("../../../api/source.json")
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, metadata
}

func TestSnapshotIntegrity(t *testing.T) {
	t.Parallel()
	snapshot, metadata := snapshotFiles(t)
	if err := verifySnapshot(snapshot, metadata); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotRejectsChangedContent(t *testing.T) {
	t.Parallel()
	snapshot, metadata := snapshotFiles(t)
	snapshot[0] ^= 1
	if err := verifySnapshot(snapshot, metadata); err == nil {
		t.Fatal("changed snapshot accepted")
	}
}

func TestSnapshotRejectsMissingChecksum(t *testing.T) {
	t.Parallel()
	snapshot, _ := snapshotFiles(t)
	if err := verifySnapshot(snapshot, []byte(`{}`)); err == nil {
		t.Fatal("missing checksum accepted")
	}
}

func TestPreparationIsDeterministicAndPreservesSnapshot(t *testing.T) {
	t.Parallel()
	snapshot, metadata := snapshotFiles(t)
	first, err := prepare(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepare(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("preparation is not deterministic")
	}
	if err := verifySnapshot(snapshot, metadata); err != nil {
		t.Fatal("preparation mutated snapshot")
	}
}

func TestPreparationRejectsAlreadyAdaptedSchema(t *testing.T) {
	t.Parallel()
	snapshot, _ := snapshotFiles(t)
	adapted, err := prepare(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepare(adapted); err == nil {
		t.Fatal("already adapted schema accepted")
	}
}

func TestChangedSchemaFailsWithoutPanic(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		input string
	}{
		{
			name:  "malformed JSON",
			input: "{",
		},
		{
			name:  "missing components",
			input: "{}",
		},
		{
			name:  "missing allOf member",
			input: `{"components":{"schemas":{"RiskRequest":{"allOf":[]}}}}`,
		},
		{
			name:  "changed required shape",
			input: `{"components":{"schemas":{"RiskRequest":{"allOf":[{},{"required":null}]}}}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := prepare([]byte(tc.input)); err == nil {
				t.Fatal("unsupported schema accepted")
			}
		})
	}
}
