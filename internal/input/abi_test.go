package input_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	inputadapter "github.com/otrumb/evm-failure-packet/internal/input"
)

func TestLoadABIs_loads_custom_error(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "contract.json")
	content := `[{"type":"error","name":"Denied","inputs":[{"name":"account","type":"address"}]}]`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := inputadapter.LoadABIs([]string{path})
	if err != nil {
		t.Fatalf("LoadABIs() error = %v", err)
	}
	if _, ok := loaded[0].ABI.Errors["Denied"]; !ok {
		t.Fatal("Denied error absent from loaded ABI")
	}
}

func TestLoadABIs_rejects_oversized_file(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "large.json")
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", inputadapter.MaxABIBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := inputadapter.LoadABIs([]string{path})
	if !errors.Is(err, inputadapter.ErrLimit) {
		t.Fatalf("LoadABIs() error = %v, want ErrLimit", err)
	}
}

func TestLoadABIs_rejects_too_many_files_before_opening(t *testing.T) {
	paths := make([]string, inputadapter.MaxABIs+1)
	_, err := inputadapter.LoadABIs(paths)
	if !errors.Is(err, inputadapter.ErrLimit) {
		t.Fatalf("LoadABIs() error = %v, want ErrLimit", err)
	}
}
