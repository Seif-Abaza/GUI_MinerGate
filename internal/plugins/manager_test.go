package plugins

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"minergate/internal/models"
)

func buildTestPlugin(t *testing.T, destDir string) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	// Resolve plugin source relative to the package directory; go test usually runs with cwd set to the package.
	srcDir := filepath.Join(wd, "..", "..", "plugins", "hello_world")

	outPath := filepath.Join(destDir, "hello_world.so")

	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", outPath, srcDir)
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	cmd.Dir = wd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, string(out))
	}
	return outPath
}

func TestLoadAndUsePlugin(t *testing.T) {
	tmpDir := t.TempDir()
	buildTestPlugin(t, tmpDir)

	m := NewManager(tmpDir)
	if err := m.LoadAll(); err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if got := m.GetPluginCount(); got != 1 {
		t.Fatalf("expected 1 plugin, got %d", got)
	}

	p, err := m.GetPlugin("hello_world")
	if err != nil {
		t.Fatalf("GetPlugin failed: %v", err)
	}
	if p.Info.Name != "hello_world" {
		t.Fatalf("unexpected plugin name: %s", p.Info.Name)
	}

	if errs := m.NotifyMinerUpdate(&models.Miner{ID: "m1"}); len(errs) != 0 {
		t.Fatalf("unexpected errors from NotifyMinerUpdate: %v", errs)
	}
	if errs := m.NotifyFarmUpdate(&models.Farm{UUID: "f1"}); len(errs) != 0 {
		t.Fatalf("unexpected errors from NotifyFarmUpdate: %v", errs)
	}
	proceed, errs := m.NotifyAction("test", "m1")
	if !proceed || len(errs) != 0 {
		t.Fatalf("unexpected result from NotifyAction: proceed=%v errs=%v", proceed, errs)
	}

	if err := m.Disable("hello_world"); err != nil {
		t.Fatalf("Disable failed: %v", err)
	}
	if got := m.GetEnabledCount(); got != 0 {
		t.Fatalf("expected 0 enabled plugins, got %d", got)
	}

	if err := m.Enable("hello_world"); err != nil {
		t.Fatalf("Enable failed: %v", err)
	}
	if got := m.GetEnabledCount(); got != 1 {
		t.Fatalf("expected 1 enabled plugin, got %d", got)
	}

	if err := m.Unload("hello_world"); err != nil {
		t.Fatalf("Unload failed: %v", err)
	}
	if got := m.GetPluginCount(); got != 0 {
		t.Fatalf("expected 0 plugins after unload, got %d", got)
	}

	if errs := m.GetErrors(); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	// Cleanup should be safe even when no plugins are loaded.
	m.Cleanup()
	if errs := m.GetErrors(); len(errs) != 0 {
		t.Fatalf("expected no errors after cleanup, got %v", errs)
	}
}
