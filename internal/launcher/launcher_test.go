package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePrefersPlayGTAV(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"PlayGTAV.exe", "GTA5_Enhanced.exe"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("MZ"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if filepath.Base(got) != "PlayGTAV.exe" {
		t.Errorf("Resolve = %q, want PlayGTAV.exe", got)
	}
}

func TestResolveFallsBackToGameExe(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "GTA5_Enhanced.exe"), []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(dir)
	if err != nil || filepath.Base(got) != "GTA5_Enhanced.exe" {
		t.Fatalf("Resolve = (%q, %v), want GTA5_Enhanced.exe", got, err)
	}
}

func TestResolveErrorsWhenNothingThere(t *testing.T) {
	if _, err := Resolve(t.TempDir()); err == nil {
		t.Error("Resolve should error when no launcher/exe is present")
	}
	if _, err := Resolve(""); err == nil {
		t.Error("Resolve should error on an empty game dir")
	}
}
