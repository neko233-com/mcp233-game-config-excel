package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadStatusAutoSelectsNewestAndRollbackPreviews(t *testing.T) {
	directory := t.TempDir()
	for _, version := range []string{"0.3.0", "0.4.0"} {
		path := filepath.Join(directory, binaryPrefix+version+binarySuffix)
		if err := os.WriteFile(path, []byte("binary"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	status, err := ReadStatus(directory)
	if err != nil || status.SelectedVersion != "0.4.0" || status.ActivePolicy != "auto" {
		t.Fatalf("unexpected status: %+v err=%v", status, err)
	}
	rollback, err := Rollback(directory, false)
	if err != nil || rollback.SelectedVersion != "0.3.0" || rollback.ActivePolicy != "0.3.0" {
		t.Fatalf("unexpected rollback preview: %+v err=%v", rollback, err)
	}
	if _, err := os.Stat(filepath.Join(directory, activeFile)); !os.IsNotExist(err) {
		t.Fatal("preview must not write active policy")
	}
	activated, err := SetActiveVersion(directory, "0.3.0", true)
	if err != nil || activated.SelectedVersion != "0.3.0" {
		t.Fatalf("unexpected activation: %+v err=%v", activated, err)
	}
}
