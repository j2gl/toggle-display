// Package profile contains monitor discovery, profile matching, and layout
// selection. It has no process or filesystem side effects.
package profile

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"toggle-display/internal/config"
	"toggle-display/internal/displayplacer"
	"toggle-display/internal/identity"
)

type Discovery struct {
	Laptop   displayplacer.Display
	External displayplacer.Display
}

// Discover identifies exactly one built-in and exactly one external display.
// A monitor is never selected arbitrarily when the hardware setup is
// ambiguous.
func Discover(snapshot displayplacer.Snapshot, laptopIDs []identity.Identifier) (Discovery, error) {
	if len(snapshot.Displays) == 0 {
		return Discovery{}, errors.New("no displays were found")
	}
	var laptops []displayplacer.Display
	for _, display := range snapshot.Displays {
		if matchesAnyID(display, laptopIDs) || looksLikeLaptop(display) {
			laptops = append(laptops, display)
		}
	}
	if len(laptops) == 0 {
		return Discovery{}, errors.New("the built-in laptop display was not found; add its persistent or serial id to laptop_ids")
	}
	if len(laptops) > 1 {
		return Discovery{}, fmt.Errorf("found %d possible laptop displays; check laptop_ids", len(laptops))
	}
	var externals []displayplacer.Display
	for _, display := range snapshot.Displays {
		if !sameDisplay(display, laptops[0]) {
			externals = append(externals, display)
		}
	}
	if len(externals) == 0 {
		return Discovery{}, errors.New("no external monitor is connected")
	}
	if len(externals) > 1 {
		return Discovery{}, fmt.Errorf("found %d external monitors; connect exactly one (multiple-monitor support is not available yet)", len(externals))
	}
	external := externals[0]
	// A serial is useful only when it identifies one screen in this snapshot.
	// Keep the persistent id as the safe fallback and make duplicate serials
	// unavailable to profile matching and future capture.
	if external.SerialID != "" && serialCount(snapshot.Displays, external.SerialID) > 1 {
		external.SerialID = ""
	}
	return Discovery{Laptop: laptops[0], External: external}, nil
}

func serialCount(displays []displayplacer.Display, serial string) int {
	count := 0
	for _, display := range displays {
		if display.SerialID != "" && identity.Normalize(display.SerialID) == identity.Normalize(serial) {
			count++
		}
	}
	return count
}

func matchesAnyID(display displayplacer.Display, ids []identity.Identifier) bool {
	for _, id := range ids {
		if !id.Useful() {
			continue
		}
		if display.HasIdentifier(id) {
			return true
		}
	}
	return false
}

func looksLikeLaptop(display displayplacer.Display) bool {
	value := strings.ToLower(display.Type + " " + display.Name)
	for _, marker := range []string{"built-in", "built in", "builtin", "internal", "retina", "color lcd", "laptop"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func sameDisplay(a, b displayplacer.Display) bool {
	for _, aID := range a.AllIdentifiers() {
		if aID.Value == "" || (aID.Type == identity.Serial && identity.IsGenericSerial(aID.Value)) {
			continue
		}
		if b.HasIdentifier(aID) {
			return true
		}
	}
	return false
}

// MatchRank returns the priority of the best identifier match. Lower is
// stronger: real serial, then persistent. Generic serial values are ignored
// entirely.
func MatchRank(p config.Profile, monitor displayplacer.Display) (int, bool) {
	best := 100
	for _, registered := range p.MonitorIDs {
		if !registered.Useful() || !monitor.HasIdentifier(registered) {
			continue
		}
		rank := 100
		switch registered.Type {
		case identity.Serial:
			rank = 0
		case identity.Persistent:
			rank = 1
		}
		if rank < best {
			best = rank
		}
	}
	return best, best != 100
}

type Candidate struct {
	Profile config.Profile
	Rank    int
}

// Candidates returns matches ordered by identifier strength and profile id.
func Candidates(profiles []config.Profile, monitor displayplacer.Display) []Candidate {
	candidates := make([]Candidate, 0)
	for _, p := range profiles {
		if rank, ok := MatchRank(p, monitor); ok {
			candidates = append(candidates, Candidate{Profile: p, Rank: rank})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Rank != candidates[j].Rank {
			return candidates[i].Rank < candidates[j].Rank
		}
		return strings.ToLower(candidates[i].Profile.ID) < strings.ToLower(candidates[j].Profile.ID)
	})
	return candidates
}

// Select chooses exactly one best profile. A strong serial match takes
// precedence over weaker persistent matches; ties are refused.
func Select(profiles []config.Profile, monitor displayplacer.Display) (config.Profile, error) {
	candidates := Candidates(profiles, monitor)
	if len(candidates) == 0 {
		return config.Profile{}, fmt.Errorf("no profile matches the connected monitor (%s)", ConnectedIDs(monitor))
	}
	bestRank := candidates[0].Rank
	best := candidates[:0]
	for _, candidate := range candidates {
		if candidate.Rank != bestRank {
			break
		}
		best = append(best, candidate)
	}
	if len(best) != 1 {
		ids := make([]string, len(best))
		for i, candidate := range best {
			ids[i] = candidate.Profile.ID
		}
		return config.Profile{}, fmt.Errorf("connected monitor matches multiple profiles (%s); use --profile explicitly or remove the duplicate monitor id", strings.Join(ids, ", "))
	}
	return best[0].Profile, nil
}

// Matches verifies an explicitly selected profile without applying a weaker
// profile as a substitute.
func Matches(p config.Profile, monitor displayplacer.Display) bool {
	_, ok := MatchRank(p, monitor)
	return ok
}

func ConnectedIDs(display displayplacer.Display) string {
	ids := display.AllIdentifiers()
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		if id.Value != "" {
			parts = append(parts, id.String())
		}
	}
	if len(parts) == 0 {
		return "no persistent or serial ids reported"
	}
	return strings.Join(parts, ", ")
}

// CaptureMonitorIDs returns useful ids in matching priority order. A generic
// serial is omitted; a profile containing only such a serial would otherwise
// match unrelated hardware.
func CaptureMonitorIDs(display displayplacer.Display) ([]identity.Identifier, error) {
	ids := identity.Ordered(
		identity.Identifier{Type: identity.Serial, Value: display.SerialID},
		identity.Identifier{Type: identity.Persistent, Value: display.PersistentID},
	)
	if len(ids) == 0 {
		return nil, errors.New("the external monitor reported no useful screen identifier; refusing to create an unmatchable profile")
	}
	return ids, nil
}

// MergeMonitorIDs keeps old identities while learning ids after a macOS
// persistent-id change.
func MergeMonitorIDs(existing []identity.Identifier, observed displayplacer.Display) ([]identity.Identifier, error) {
	captured, err := CaptureMonitorIDs(observed)
	if err != nil {
		return nil, err
	}
	result := append([]identity.Identifier(nil), existing...)
	seen := make(map[string]bool, len(result))
	for _, id := range result {
		seen[id.Key()] = true
	}
	for _, id := range captured {
		if !seen[id.Key()] {
			result = append(result, id)
			seen[id.Key()] = true
		}
	}
	// Keep the preferred real ids first without changing the relative order
	// between identifiers of the same type.
	return identity.Ordered(result...), nil
}

// CaptureArguments converts the current arrangement to role placeholders.
func CaptureArguments(snapshot displayplacer.Snapshot, laptop, external displayplacer.Display) ([]string, error) {
	if len(snapshot.Arrangement) == 0 {
		return nil, errors.New("displayplacer list did not contain its current arrangement command")
	}
	args := make([]string, 0, len(snapshot.Arrangement))
	for _, spec := range snapshot.Arrangement {
		role := ""
		if displayContainsValue(laptop, spec.ID) {
			role = "LAPTOP_ID"
		} else if displayContainsValue(external, spec.ID) {
			role = "EXTERNAL_ID"
		}
		if role == "" {
			return nil, fmt.Errorf("current arrangement references unknown screen id %q", spec.ID)
		}
		arg, err := displayplacer.ReplaceSpecID(spec.Raw, role)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	return args, nil
}

func displayContainsValue(display displayplacer.Display, value string) bool {
	for _, id := range display.AllIdentifiers() {
		if id.Value == "" || (id.Type == identity.Serial && identity.IsGenericSerial(id.Value)) {
			continue
		}
		if identity.Normalize(id.Value) == identity.Normalize(value) {
			return true
		}
	}
	return false
}

// SubstituteArguments replaces role placeholders with ids from the current
// discovery. It passes each specification as one exec argument and never
// invokes a shell.
func SubstituteArguments(args []string, laptop, external displayplacer.Display) ([]string, error) {
	laptopID, externalID := laptop.PreferredID(), external.PreferredID()
	if laptopID == "" || externalID == "" {
		return nil, errors.New("cannot apply layout: a connected screen has no usable id")
	}
	result := make([]string, len(args))
	hasLaptop, hasExternal := false, false
	for i, arg := range args {
		spec, err := displayplacer.ParseSpec(arg)
		if err != nil {
			return nil, fmt.Errorf("layout argument %d: %w", i+1, err)
		}
		replacement := ""
		switch spec.ID {
		case "LAPTOP_ID":
			replacement = laptopID
			hasLaptop = true
		case "EXTERNAL_ID":
			replacement = externalID
			hasExternal = true
		}
		if replacement != "" {
			result[i], err = displayplacer.ReplaceSpecID(arg, replacement)
			if err != nil {
				return nil, err
			}
		} else {
			result[i] = arg
		}
	}
	if !hasLaptop || !hasExternal {
		return nil, errors.New("layout must contain both LAPTOP_ID and EXTERNAL_ID")
	}
	return result, nil
}

// DetectLayout returns the ordered index of the active layout, or -1 when the
// current arrangement is not recognized. The laptop origin is intentionally
// the first discriminator: it is stable, easy to inspect, and validated as
// unique for every profile.
func DetectLayout(p config.Profile, laptop displayplacer.Display) (int, bool, error) {
	for i, id := range p.LayoutOrder {
		layout, ok := config.LayoutByID(p, id)
		if !ok {
			return -1, false, fmt.Errorf("profile %q has invalid layout order entry %q", p.ID, id)
		}
		x, y, err := config.ParseOrigin(layout.LaptopOrigin)
		if err != nil {
			return -1, false, err
		}
		if laptop.Origin.X == x && laptop.Origin.Y == y {
			return i, true, nil
		}
	}
	return -1, false, nil
}

// NextLayout chooses the next explicit layout, falling back to the first when
// the current arrangement is unknown.
func NextLayout(p config.Profile, laptop displayplacer.Display) (config.Layout, bool, error) {
	if len(p.LayoutOrder) == 0 {
		return config.Layout{}, false, fmt.Errorf("profile %q has no layouts; save one with --save", p.ID)
	}
	if len(p.LayoutOrder) == 1 {
		return config.Layout{}, false, fmt.Errorf("profile %q has only one layout; save another layout before toggling", p.ID)
	}
	index, known, err := DetectLayout(p, laptop)
	if err != nil {
		return config.Layout{}, false, err
	}
	next := 0
	if known {
		next = (index + 1) % len(p.LayoutOrder)
	}
	layout, ok := config.LayoutByID(p, p.LayoutOrder[next])
	if !ok {
		return config.Layout{}, false, fmt.Errorf("profile %q layout order is invalid", p.ID)
	}
	return layout, known, nil
}
