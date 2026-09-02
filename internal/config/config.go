// Package config owns the versioned, user-editable JSON configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"toggle-display/internal/identity"
)

const CurrentVersion = 1

// Identifier is re-exported so callers do not need to know about the shared
// identity package when constructing configuration values.
type Identifier = identity.Identifier

type Profile struct {
	ID          string                `json:"id"`
	Tags        map[string]string     `json:"tags,omitempty"`
	MonitorIDs  []identity.Identifier `json:"monitor_ids"`
	LayoutOrder []string              `json:"layout_order"`
	Layouts     []Layout              `json:"layouts"`
}

type Layout struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description,omitempty"`
	LaptopOrigin      string   `json:"laptop_origin"`
	DisplayplacerArgs []string `json:"displayplacer_args"`
}

type Config struct {
	Version   int                   `json:"version"`
	LaptopIDs []identity.Identifier `json:"laptop_ids"`
	Profiles  []Profile             `json:"profiles"`
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
var tagKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
var originPattern = regexp.MustCompile(`^\(\s*-?[0-9]+\s*,\s*-?[0-9]+\s*\)$`)

// Default returns a new configuration with the known built-in display ids.
// These ids are only used to identify the laptop; monitor profiles are created
// explicitly with --save or imported with --migrate.
func Default() Config {
	return Config{
		Version: CurrentVersion,
		LaptopIDs: []identity.Identifier{
			{Type: identity.Serial, Value: "s4251086178"},
			{Type: identity.Persistent, Value: "37D8832A-2D66-02CA-B9F7-8F30A301B230"},
		},
		Profiles: []Profile{},
	}
}

// Validate checks all invariants needed before a configuration is used or
// written. In particular, layout order is explicit and complete.
func (c Config) Validate() error {
	if c.Version != CurrentVersion {
		return fmt.Errorf("unsupported config version %d (expected %d)", c.Version, CurrentVersion)
	}
	if err := validateIdentifiers("laptop_ids", c.LaptopIDs); err != nil {
		return err
	}

	profiles := make(map[string]bool)
	monitorIDs := make(map[string]string)
	for pi, p := range c.Profiles {
		if err := validateID("profile id", p.ID); err != nil {
			return fmt.Errorf("profile %d: %w", pi+1, err)
		}
		if profiles[strings.ToLower(p.ID)] {
			return fmt.Errorf("duplicate profile id %q", p.ID)
		}
		profiles[strings.ToLower(p.ID)] = true
		if len(p.MonitorIDs) == 0 {
			return fmt.Errorf("profile %q has no monitor ids", p.ID)
		}
		if err := validateIdentifiers("monitor ids for profile "+p.ID, p.MonitorIDs); err != nil {
			return err
		}
		usefulID := false
		for _, id := range p.MonitorIDs {
			if id.Useful() {
				usefulID = true
			}
		}
		if !usefulID {
			return fmt.Errorf("profile %q has no useful monitor id; a generic serial cannot be its only identity", p.ID)
		}
		for _, id := range p.MonitorIDs {
			// Generic serials are deliberately not profile identities. Allowing
			// them in JSON would make two otherwise unrelated monitors match.
			if !id.Useful() {
				continue
			}
			key := id.Key()
			if other, ok := monitorIDs[key]; ok {
				return fmt.Errorf("monitor id %s is registered by both profiles %q and %q", id, other, p.ID)
			}
			monitorIDs[key] = p.ID
		}
		for key, value := range p.Tags {
			if !tagKeyPattern.MatchString(key) {
				return fmt.Errorf("profile %q has invalid tag key %q", p.ID, key)
			}
			if err := validateText(value, "tag "+key); err != nil {
				return fmt.Errorf("profile %q: %w", p.ID, err)
			}
		}
		if err := validateLayouts(p); err != nil {
			return err
		}
	}
	return nil
}

func validateLayouts(p Profile) error {
	layouts := make(map[string]bool, len(p.Layouts))
	origins := make(map[string]string, len(p.Layouts))
	for i, layout := range p.Layouts {
		if err := validateID("layout id", layout.ID); err != nil {
			return fmt.Errorf("profile %q layout %d: %w", p.ID, i+1, err)
		}
		if layouts[strings.ToLower(layout.ID)] {
			return fmt.Errorf("profile %q has duplicate layout id %q", p.ID, layout.ID)
		}
		layouts[strings.ToLower(layout.ID)] = true
		if layout.Name == "" {
			return fmt.Errorf("profile %q layout %q has an empty name", p.ID, layout.ID)
		}
		if err := validateText(layout.Name, "layout name"); err != nil {
			return fmt.Errorf("profile %q layout %q: %w", p.ID, layout.ID, err)
		}
		if err := validateText(layout.Description, "layout description"); err != nil {
			return fmt.Errorf("profile %q layout %q: %w", p.ID, layout.ID, err)
		}
		if !originPattern.MatchString(strings.TrimSpace(layout.LaptopOrigin)) {
			return fmt.Errorf("profile %q layout %q has invalid laptop_origin %q", p.ID, layout.ID, layout.LaptopOrigin)
		}
		origin := normalizeOrigin(layout.LaptopOrigin)
		if other, ok := origins[origin]; ok {
			return fmt.Errorf("profile %q layouts %q and %q have the same laptop origin", p.ID, other, layout.ID)
		}
		origins[origin] = layout.ID
		if len(layout.DisplayplacerArgs) == 0 {
			return fmt.Errorf("profile %q layout %q has no displayplacer arguments", p.ID, layout.ID)
		}
		hasLaptop, hasExternal := false, false
		for _, arg := range layout.DisplayplacerArgs {
			if strings.TrimSpace(arg) == "" || strings.ContainsAny(arg, "\r\n\x00") {
				return fmt.Errorf("profile %q layout %q has an invalid displayplacer argument", p.ID, layout.ID)
			}
			hasLaptop = hasLaptop || strings.Contains(arg, "id:LAPTOP_ID")
			hasExternal = hasExternal || strings.Contains(arg, "id:EXTERNAL_ID")
		}
		if !hasLaptop || !hasExternal {
			return fmt.Errorf("profile %q layout %q must contain both LAPTOP_ID and EXTERNAL_ID", p.ID, layout.ID)
		}
	}
	if len(p.LayoutOrder) != len(p.Layouts) {
		return fmt.Errorf("profile %q layout_order must contain every layout exactly once", p.ID)
	}
	seenOrder := make(map[string]bool, len(p.LayoutOrder))
	for _, id := range p.LayoutOrder {
		if !layouts[strings.ToLower(id)] {
			return fmt.Errorf("profile %q layout_order refers to unknown layout %q", p.ID, id)
		}
		if seenOrder[strings.ToLower(id)] {
			return fmt.Errorf("profile %q layout_order contains duplicate layout %q", p.ID, id)
		}
		seenOrder[strings.ToLower(id)] = true
	}
	return nil
}

func validateIdentifiers(label string, ids []identity.Identifier) error {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id.Type != identity.Serial && id.Type != identity.Persistent && id.Type != identity.Contextual {
			return fmt.Errorf("%s has unsupported identifier type %q", label, id.Type)
		}
		if strings.TrimSpace(id.Value) == "" {
			return fmt.Errorf("%s contains an empty identifier", label)
		}
		key := id.Key()
		if seen[key] {
			return fmt.Errorf("%s contains duplicate identifier %s", label, id)
		}
		seen[key] = true
	}
	return nil
}

func validateID(label, value string) error {
	if !idPattern.MatchString(value) {
		return fmt.Errorf("invalid %s %q (use letters, numbers, '.', '_' or '-', starting with a letter or number)", label, value)
	}
	return nil
}

func validateText(value, label string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s contains invalid text", label)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s contains control characters", label)
		}
	}
	return nil
}

func normalizeOrigin(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), "")
}

// Load reads path without executing or interpreting any configuration as
// shell. A missing file returns the safe default configuration.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return c, nil
}

// Save writes a configuration atomically. The temporary file is in the same
// directory, is synced and closed before rename, and is private to the user.
func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("refusing to write invalid config: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config.json-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replace config: %w", err)
	}
	// Syncing the directory makes the rename durable on filesystems that
	// support directory fsync. It is best effort on platforms that reject it.
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}
	return nil
}

func FindProfile(c *Config, id string) (*Profile, error) {
	for i := range c.Profiles {
		if strings.EqualFold(c.Profiles[i].ID, id) {
			return &c.Profiles[i], nil
		}
	}
	return nil, fmt.Errorf("profile %q not found", id)
}

func FindLayout(p *Profile, id string) (*Layout, error) {
	for i := range p.Layouts {
		if strings.EqualFold(p.Layouts[i].ID, id) {
			return &p.Layouts[i], nil
		}
	}
	return nil, fmt.Errorf("profile %q has no layout %q", p.ID, id)
}

// LayoutByID returns a layout without exposing pointer-to-range pitfalls.
func LayoutByID(p Profile, id string) (Layout, bool) {
	for _, layout := range p.Layouts {
		if strings.EqualFold(layout.ID, id) {
			return layout, true
		}
	}
	return Layout{}, false
}

// SortedProfiles is useful only for presentation; layout order is never
// sorted and remains the user's explicit order.
func SortedProfiles(profiles []Profile) []Profile {
	result := append([]Profile(nil), profiles...)
	sort.SliceStable(result, func(i, j int) bool { return strings.ToLower(result[i].ID) < strings.ToLower(result[j].ID) })
	return result
}

// ConfigPath returns the standard path described by the command design.
func ConfigPath(home, xdgConfigHome string) string {
	base := xdgConfigHome
	if base == "" {
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "toggle-display", "config.json")
}

// ParseOrigin validates and returns the coordinates in a layout's origin.
func ParseOrigin(value string) (x, y int, err error) {
	if !originPattern.MatchString(strings.TrimSpace(value)) {
		return 0, 0, fmt.Errorf("invalid origin %q", value)
	}
	_, err = fmt.Sscanf(strings.TrimSpace(value), "(%d,%d)", &x, &y)
	if err != nil {
		// Sscanf accepts no spaces with this format, while validation permits
		// them. Parse the numbers after removing whitespace.
		compact := strings.Join(strings.Fields(value), "")
		_, err = fmt.Sscanf(compact, "(%d,%d)", &x, &y)
	}
	return x, y, err
}
