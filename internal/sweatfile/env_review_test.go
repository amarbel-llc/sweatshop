package sweatfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".sweatshop-env")
	os.WriteFile(envPath, []byte("EDITOR=nvim\n"), 0o644)

	err := SnapshotEnv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".sweatshop-env.snapshot"))
	if string(data) != "EDITOR=nvim\n" {
		t.Errorf("snapshot: got %q", string(data))
	}
}

func TestSnapshotEnvMissing(t *testing.T) {
	dir := t.TempDir()
	err := SnapshotEnv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = os.Stat(filepath.Join(dir, ".sweatshop-env.snapshot"))
	if err == nil {
		t.Error("expected no snapshot for missing env file")
	}
}

func TestDiffEnv(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".sweatshop-env.snapshot"), []byte("EDITOR=nvim\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".sweatshop-env"), []byte("EDITOR=nvim\nPAGER=less\n"), 0o644)

	added, changed, err := DiffEnv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(added) != 1 || added["PAGER"] != "less" {
		t.Errorf("added: got %v", added)
	}
	if len(changed) != 0 {
		t.Errorf("changed: got %v", changed)
	}
}

func TestDiffEnvChanged(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".sweatshop-env.snapshot"), []byte("EDITOR=vim\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".sweatshop-env"), []byte("EDITOR=nvim\n"), 0o644)

	_, changed, err := DiffEnv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 1 || changed["EDITOR"] != "nvim" {
		t.Errorf("changed: got %v", changed)
	}
}

func TestCleanupEnvSnapshot(t *testing.T) {
	dir := t.TempDir()
	snap := filepath.Join(dir, ".sweatshop-env.snapshot")
	os.WriteFile(snap, []byte("x"), 0o644)
	CleanupEnvSnapshot(dir)
	if _, err := os.Stat(snap); err == nil {
		t.Error("expected snapshot removed")
	}
}
