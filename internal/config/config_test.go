package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"toggle-display/internal/identity"
)

func testConfig() Config {
	return Config{
		Version:   CurrentVersion,
		LaptopIDs: []identity.Identifier{{Type: identity.Persistent, Value: "laptop"}},
		Profiles: []Profile{{
			ID:          "hp-home",
			Tags:        map[string]string{"place": "home"},
			MonitorIDs:  []identity.Identifier{{Type: identity.Persistent, Value: "monitor"}},
			LayoutOrder: []string{"layout1", "layout2"},
			Layouts: []Layout{
				{ID: "layout1", Name: "next to laptop", Description: "left side", LaptopOrigin: "(0, 0)", DisplayplacerArgs: []string{"id:EXTERNAL_ID res:1x1", "id:LAPTOP_ID res:1x1"}},
				{ID: "layout2", Name: "under-monitor", Description: "right side: punctuation!", LaptopOrigin: "(10,20)", DisplayplacerArgs: []string{"id:EXTERNAL_ID res:1x1", "id:LAPTOP_ID res:1x1"}},
			},
		}},
	}
}

func TestSaveLoadAtomicJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.json")
	want := testConfig()
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles[0].Layouts[1].Description != want.Profiles[0].Layouts[1].Description {
		t.Fatalf("description was not preserved: %+v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"version": 1`) || strings.Contains(string(data), "LAYOUT_") {
		t.Fatalf("unexpected JSON: %s", data)
	}
	if mode := fileMode(t, path); mode&0077 != 0 {
		t.Fatalf("config is not private: %v", mode)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode()
}

func TestValidationRejectsDuplicateOrderAndOrigin(t *testing.T) {
	c := testConfig()
	c.Profiles[0].LayoutOrder = []string{"layout1", "layout1"}
	if err := c.Validate(); err == nil {
		t.Fatal("duplicate layout order was accepted")
	}
	c = testConfig()
	c.Profiles[0].Layouts[1].LaptopOrigin = "(0,0)"
	if err := c.Validate(); err == nil {
		t.Fatal("duplicate origins were accepted")
	}
}

func TestGenericSerialCannotBeOnlyProfileIdentity(t *testing.T) {
	c := testConfig()
	c.Profiles[0].MonitorIDs = []identity.Identifier{{Type: identity.Serial, Value: "s0"}}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "generic serial") {
		t.Fatalf("generic-only profile was accepted: %v", err)
	}
}

func TestMissingConfigReturnsDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Version != CurrentVersion || len(c.LaptopIDs) == 0 {
		t.Fatalf("unexpected default config: %+v", c)
	}
}
