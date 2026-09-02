package profile

import (
	"strings"
	"testing"

	"toggle-display/internal/config"
	"toggle-display/internal/displayplacer"
	"toggle-display/internal/identity"
)

func displays() (displayplacer.Display, displayplacer.Display) {
	return displayplacer.Display{
			PersistentID: "laptop-persistent", SerialID: "s-laptop",
			Type: "Built-in Retina Display", Origin: displayplacer.Point{X: 10, Y: 20},
		}, displayplacer.Display{
			PersistentID: "monitor-persistent", SerialID: "s12345",
			Type: "external", Origin: displayplacer.Point{},
		}
}

func twoLayoutProfile() config.Profile {
	return config.Profile{
		ID:          "monitor-profile",
		MonitorIDs:  []identity.Identifier{{Type: identity.Persistent, Value: "monitor-persistent"}},
		LayoutOrder: []string{"one", "two"},
		Layouts: []config.Layout{
			{ID: "one", Name: "next-to-laptop", LaptopOrigin: "(0,0)", DisplayplacerArgs: []string{`id:EXTERNAL_ID res:1x1`, `id:LAPTOP_ID res:1x1 origin:(0,0)`}},
			{ID: "two", Name: "under monitor", LaptopOrigin: "(10,20)", DisplayplacerArgs: []string{`id:EXTERNAL_ID res:1x1`, `id:LAPTOP_ID res:1x1 origin:(10,20)`}},
		},
	}
}

func TestSelectPrefersRealSerialAndRejectsTies(t *testing.T) {
	_, external := displays()
	serial := config.Profile{ID: "serial", MonitorIDs: []identity.Identifier{{Type: identity.Serial, Value: "s12345"}}}
	persistent := config.Profile{ID: "persistent", MonitorIDs: []identity.Identifier{{Type: identity.Persistent, Value: "monitor-persistent"}}}
	selected, err := Select([]config.Profile{persistent, serial}, external)
	if err != nil || selected.ID != "serial" {
		t.Fatalf("serial profile was not preferred: %v %+v", err, selected)
	}
	first := config.Profile{ID: "first", MonitorIDs: []identity.Identifier{{Type: identity.Serial, Value: "s12345"}}}
	second := config.Profile{ID: "second", MonitorIDs: []identity.Identifier{{Type: identity.Serial, Value: "s12345"}}}
	external.PersistentID = "monitor-persistent"
	// Both profiles intentionally share the observed serial, creating a real
	// ambiguity even though their persistent ids are not involved.
	if _, err := Select([]config.Profile{first, second}, external); err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Fatalf("ambiguous match was accepted: %v", err)
	}
}

func TestGenericSerialIsIgnored(t *testing.T) {
	_, external := displays()
	external.SerialID = "s0"
	onlySerial := config.Profile{ID: "generic", MonitorIDs: []identity.Identifier{{Type: identity.Serial, Value: "s0"}}}
	if Matches(onlySerial, external) {
		t.Fatal("generic serial matched a profile")
	}
	if _, err := CaptureMonitorIDs(external); err != nil {
		t.Fatal("persistent fallback should make capture possible:", err)
	}
}

func TestDiscoverAndCycle(t *testing.T) {
	laptop, external := displays()
	snapshot := displayplacer.Snapshot{Displays: []displayplacer.Display{external, laptop}}
	discovered, err := Discover(snapshot, []identity.Identifier{{Type: identity.Persistent, Value: "laptop-persistent"}})
	if err != nil || discovered.External.PersistentID != external.PersistentID {
		t.Fatalf("discovery failed: %v %+v", err, discovered)
	}
	p := twoLayoutProfile()
	laptop.Origin = displayplacer.Point{X: 0, Y: 0}
	layout, known, err := NextLayout(p, laptop)
	if err != nil || !known || layout.ID != "two" {
		t.Fatalf("next layout from first failed: %v %v %+v", err, known, layout)
	}
	laptop.Origin = displayplacer.Point{X: 99, Y: 99}
	layout, known, err = NextLayout(p, laptop)
	if err != nil || known || layout.ID != "one" {
		t.Fatalf("unknown layout should fall back to first: %v %v %+v", err, known, layout)
	}
}

func TestLayoutCountRules(t *testing.T) {
	laptop, _ := displays()
	one := twoLayoutProfile()
	one.LayoutOrder = []string{"one"}
	one.Layouts = one.Layouts[:1]
	if _, _, err := NextLayout(one, laptop); err == nil || !strings.Contains(err.Error(), "only one") {
		t.Fatalf("one-layout profile was toggled: %v", err)
	}
	three := twoLayoutProfile()
	three.Layouts = append(three.Layouts, config.Layout{ID: "three", Name: "third", LaptopOrigin: "(30,40)", DisplayplacerArgs: []string{"id:EXTERNAL_ID"}})
	three.LayoutOrder = append(three.LayoutOrder, "three")
	laptop.Origin = displayplacer.Point{X: 10, Y: 20}
	layout, known, err := NextLayout(three, laptop)
	if err != nil || !known || layout.ID != "three" {
		t.Fatalf("three-layout cycle failed: %v %v %+v", err, known, layout)
	}
}

func TestCaptureAndSubstituteWithoutShell(t *testing.T) {
	laptop, external := displays()
	externalSpec := `id:monitor-persistent res:2560x1440 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0`
	laptopSpec := `id:laptop-persistent res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(10,20) degree:0`
	snapshot := displayplacer.Snapshot{Arrangement: []displayplacer.ScreenSpec{
		{ID: external.PersistentID, Raw: externalSpec}, {ID: laptop.PersistentID, Raw: laptopSpec},
	}}
	args, err := CaptureArguments(snapshot, laptop, external)
	if err != nil || !strings.Contains(args[0], "EXTERNAL_ID") || !strings.Contains(args[1], "LAPTOP_ID") {
		t.Fatalf("capture failed: %v %#v", err, args)
	}
	applied, err := SubstituteArguments(args, laptop, external)
	if err != nil || !strings.Contains(applied[0], "id:s12345") || !strings.Contains(applied[1], "id:s-laptop") {
		t.Fatalf("substitution failed: %v %#v", err, applied)
	}
}
