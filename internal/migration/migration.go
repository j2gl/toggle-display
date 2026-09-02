// Package migration provides the one-time import of the old HP/DELL Bash
// layouts. It parses assignments as data; it never sources the Bash file.
package migration

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"toggle-display/internal/config"
	"toggle-display/internal/displayplacer"
	"toggle-display/internal/identity"
)

var assignmentRE = regexp.MustCompile(`^LAYOUT_([A-Za-z0-9_.-]+)_MONITOR_([A-Za-z0-9_]+)_(TEMPLATE|ORIGIN|DESC)\s*=\s*(.*)$`)

// BuiltinConfig transfers the layouts and ids that were compiled into the
// Bash implementation. A-D remain usable here as a backwards-compatible
// migration aid; newly saved layouts can use any valid id.
func BuiltinConfig() config.Config {
	c := config.Default()
	c.Profiles = []config.Profile{
		{
			ID:   "hp_home",
			Tags: map[string]string{"model": "HP display", "place": "unknown"},
			MonitorIDs: []identity.Identifier{
				{Type: identity.Persistent, Value: "06821F68-21CC-4370-8CC0-BE95ACB3AC1C"},
				{Type: identity.Persistent, Value: "3C4D0074-0F3D-47DD-AECB-1B80731B9B3F"},
				{Type: identity.Persistent, Value: "6F9FB1D9-2284-44F6-8357-9B84666EEBD5"},
			},
			LayoutOrder: []string{"A", "B"},
			Layouts: []config.Layout{
				{ID: "A", Name: "side-by-side", Description: "HP Layout A: Side by side", LaptopOrigin: "(2560,458)", DisplayplacerArgs: []string{
					`id:EXTERNAL_ID res:2560x1440 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0`,
					`id:LAPTOP_ID res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(2560,458) degree:0`,
				}},
				{ID: "B", Name: "under-monitor", Description: "HP Layout B: Laptop under monitor", LaptopOrigin: "(542,1440)", DisplayplacerArgs: []string{
					`id:EXTERNAL_ID res:2560x1440 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0`,
					`id:LAPTOP_ID res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(542,1440) degree:0`,
				}},
			},
		},
		{
			ID:   "dell_office",
			Tags: map[string]string{"model": "Dell display", "place": "unknown"},
			MonitorIDs: []identity.Identifier{
				{Type: identity.Serial, Value: "s1093808706"},
				{Type: identity.Persistent, Value: "4BBE0CEB-FD34-4B58-AA4B-B701217F35EA"},
				{Type: identity.Persistent, Value: "0E27842A-238C-4F65-80FB-4919641DB78B"},
			},
			LayoutOrder: []string{"C", "D"},
			Layouts: []config.Layout{
				{ID: "C", Name: "side-by-side", Description: "DELL Layout C: Side by side, laptop on the left with 144Hz", LaptopOrigin: "(0,0)", DisplayplacerArgs: []string{
					`id:LAPTOP_ID res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0`,
					`id:EXTERNAL_ID res:2560x1440 hz:144 color_depth:8 enabled:true scaling:off origin:(1512,-458) degree:0`,
				}},
				{ID: "D", Name: "under-monitor", Description: "DELL Layout D: Laptop under monitor with 144Hz", LaptopOrigin: "(524,1440)", DisplayplacerArgs: []string{
					`id:EXTERNAL_ID res:2560x1440 hz:144 color_depth:8 enabled:true scaling:off origin:(0,0) degree:0`,
					`id:LAPTOP_ID res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(524,1440) degree:0`,
				}},
			},
		},
	}
	return c
}

type legacyLayout struct {
	profile   string
	monitorID string
	layoutID  string
	template  string
	origin    string
	desc      string
}

// ImportLegacy adds saved monitor-specific assignments from path to base. A
// missing legacy file is fine: the built-in layouts are still returned.
func ImportLegacy(base config.Config, path string) (config.Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return base, nil
	}
	if err != nil {
		return config.Config{}, fmt.Errorf("read legacy config %s: %w", path, err)
	}
	records, err := parseLegacy(string(data))
	if err != nil {
		return config.Config{}, err
	}
	for _, record := range records {
		if record.template == "" || record.origin == "" || record.monitorID == "" {
			// Without the old monitor comment there is no safe profile to
			// attach this arrangement to.
			continue
		}
		args, err := displayplacer.ParseArrangementCommand(record.template)
		if err != nil {
			return config.Config{}, fmt.Errorf("legacy layout %s: %w", record.layoutID, err)
		}
		profile := findOrCreateProfile(&base, record.profile, record.monitorID)
		if record.monitorID != "" {
			appendIdentifier(profile, identity.Identifier{Type: identity.Persistent, Value: record.monitorID})
		}
		layout := config.Layout{
			ID: record.layoutID, Name: record.layoutID, Description: record.desc,
			LaptopOrigin: record.origin, DisplayplacerArgs: args,
		}
		if layout.Description == "" {
			layout.Description = "migrated from layouts.conf"
		}
		updateLayout(profile, layout)
	}
	return base, nil
}

func parseLegacy(data string) ([]legacyLayout, error) {
	records := make(map[string]*legacyLayout)
	currentType, currentMonitor := "", ""
	commentRE := regexp.MustCompile(`^#\s*([A-Za-z]+)\s+monitor persistent id:\s*(\S+)`)
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := commentRE.FindStringSubmatch(line); len(matches) == 3 {
			currentType, currentMonitor = strings.ToLower(matches[1]), matches[2]
			continue
		}
		matches := assignmentRE.FindStringSubmatch(line)
		if len(matches) != 5 {
			continue
		}
		value, err := parseShellString(matches[4])
		if err != nil {
			return nil, fmt.Errorf("parse legacy assignment: %w", err)
		}
		key := matches[2] + ":" + matches[1]
		record := records[key]
		if record == nil {
			profileName := legacyProfileName(currentType, matches[2])
			record = &legacyLayout{profile: profileName, monitorID: currentMonitor, layoutID: matches[1]}
			records[key] = record
		}
		switch matches[3] {
		case "TEMPLATE":
			record.template = value
		case "ORIGIN":
			record.origin = value
		case "DESC":
			record.desc = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read legacy config: %w", err)
	}
	result := make([]legacyLayout, 0, len(records))
	for _, record := range records {
		result = append(result, *record)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].profile != result[j].profile {
			return result[i].profile < result[j].profile
		}
		return result[i].layoutID < result[j].layoutID
	})
	return result, nil
}

func legacyProfileName(displayType, key string) string {
	switch strings.ToLower(displayType) {
	case "hp":
		return "hp_home"
	case "dell":
		return "dell_office"
	default:
		name := "migrated_" + strings.Trim(key, "_")
		if name == "migrated_" {
			return "migrated_monitor"
		}
		return name
	}
}

// parseShellString supports only the value forms emitted by the old writer.
// It intentionally rejects command substitutions and other shell syntax.
func parseShellString(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return "", errors.New("legacy value is not quoted")
	}
	quote := value[0]
	if (quote != '\'' && quote != '"') || value[len(value)-1] != quote {
		return "", errors.New("legacy value must be single- or double-quoted")
	}
	body := value[1 : len(value)-1]
	if quote == '\'' {
		// The old script writes literal single quotes as the standard shell
		// sequence '\''; decode it without evaluating anything.
		body = strings.ReplaceAll(body, "'\\''", "'")
		if strings.ContainsAny(body, "\r\n\x00") {
			return "", errors.New("legacy value contains invalid control characters")
		}
		return body, nil
	}
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		if body[i] == '\\' {
			if i+1 >= len(body) {
				return "", errors.New("unterminated escape in legacy value")
			}
			i++
		}
		b.WriteByte(body[i])
	}
	return b.String(), nil
}

func findOrCreateProfile(c *config.Config, name, monitorID string) *config.Profile {
	for i := range c.Profiles {
		if strings.EqualFold(c.Profiles[i].ID, name) {
			return &c.Profiles[i]
		}
	}
	p := config.Profile{ID: name, Tags: map[string]string{"migrated_from": "layouts.conf"}}
	if monitorID != "" {
		p.MonitorIDs = []identity.Identifier{{Type: identity.Persistent, Value: monitorID}}
	}
	c.Profiles = append(c.Profiles, p)
	return &c.Profiles[len(c.Profiles)-1]
}

func appendIdentifier(p *config.Profile, id identity.Identifier) {
	for _, existing := range p.MonitorIDs {
		if existing.Key() == id.Key() {
			return
		}
	}
	p.MonitorIDs = append(p.MonitorIDs, id)
}

func updateLayout(p *config.Profile, layout config.Layout) {
	for i := range p.Layouts {
		if strings.EqualFold(p.Layouts[i].ID, layout.ID) {
			p.Layouts[i] = layout
			return
		}
	}
	// The legacy format allowed arbitrary names for arrangements. The first
	// Go discriminator is the laptop origin, so retain the existing compatible
	// layout rather than creating an invalid duplicate-origin profile.
	for _, existing := range p.Layouts {
		ex, ey, eerr := config.ParseOrigin(existing.LaptopOrigin)
		lx, ly, lerr := config.ParseOrigin(layout.LaptopOrigin)
		if eerr == nil && lerr == nil && ex == lx && ey == ly {
			return
		}
	}
	p.Layouts = append(p.Layouts, layout)
	p.LayoutOrder = append(p.LayoutOrder, layout.ID)
}
