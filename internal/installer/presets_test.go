package installer

import (
	"testing"

	"github.com/caiojohnston/gta-mod-manager/internal/model"
)

func approvedDests(files []ProposedFile) map[string]string {
	out := map[string]string{}
	for _, f := range files {
		if f.Approved {
			out[f.RelInArchive] = f.Dest
		}
	}
	return out
}

func TestScriptHookVPreset(t *testing.T) {
	files := []ProposedFile{
		{RelInArchive: "bin/ScriptHookV.dll"},
		{RelInArchive: "bin/dinput8.dll"},
		{RelInArchive: "bin/xinput1_4.dll"},
		{RelInArchive: "bin/NativeTrainer.asi"},
		{RelInArchive: "bin/args.txt"},
		{RelInArchive: "bin/ScriptHookV.lib"},
		{RelInArchive: "ReadMe.txt"},
	}
	ScriptHookVPreset().Apply(files)

	got := approvedDests(files)
	if len(got) != 5 {
		t.Fatalf("approved %d files, want 5: %v", len(got), got)
	}
	if got["bin/ScriptHookV.dll"] != "ScriptHookV.dll" || got["bin/dinput8.dll"] != "dinput8.dll" {
		t.Errorf("core files land in root by basename: %v", got)
	}
	if got["bin/args.txt"] != "args.txt" {
		t.Error("args.txt must be installed (disables Enhanced anticheat for .asi loading)")
	}
	if _, ok := got["ReadMe.txt"]; ok {
		t.Error("ReadMe.txt should not be approved")
	}
	for _, f := range files {
		if f.RelInArchive == "bin/ScriptHookV.dll" && f.Kind != model.KindRoot {
			t.Errorf("ScriptHookV.dll kind = %q, want root", f.Kind)
		}
	}
}

func TestOpenRPFPreset(t *testing.T) {
	files := []ProposedFile{
		{RelInArchive: "OpenIV.asi"},
		{RelInArchive: "dsound.dll"},
		{RelInArchive: "dinput8.dll"},
		{RelInArchive: "How to install.txt"},
	}
	OpenRPFPreset().Apply(files)

	got := approvedDests(files)
	if len(got) != 1 || got["OpenIV.asi"] != "OpenIV.asi" {
		t.Fatalf("OpenRPF preset should approve only OpenIV.asi, got %v", got)
	}
}

func TestOpenRPFPresetAcceptsOpenRPFAsi(t *testing.T) {
	files := []ProposedFile{
		{RelInArchive: "OpenRPF.asi"},
		{RelInArchive: "readme.txt"},
	}
	OpenRPFPreset().Apply(files)
	got := approvedDests(files)
	if len(got) != 1 || got["OpenRPF.asi"] != "OpenRPF.asi" {
		t.Fatalf("preset should approve OpenRPF.asi as-is, got %v", got)
	}
}
