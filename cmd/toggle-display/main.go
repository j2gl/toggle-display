package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"toggle-display/internal/completion"
	"toggle-display/internal/config"
	"toggle-display/internal/displayplacer"
	"toggle-display/internal/identity"
	"toggle-display/internal/migration"
	"toggle-display/internal/profile"
)

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("value must not be empty")
	}
	*s = append(*s, value)
	return nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, displayplacer.NewCommandRunner("displayplacer"), "", ""))
}

func run(args []string, stdout, stderr io.Writer, runner displayplacer.Runner, defaultConfigPath, defaultLegacyPath string) int {
	fs := flag.NewFlagSet("toggle-display", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printUsage(stderr) }
	profileID := fs.String("profile", "", "profile to use (optional for toggle)")
	saveID := fs.String("save", "", "save the current arrangement under this layout id")
	applyID := fs.String("apply", "", "apply this layout id")
	name := fs.String("name", "", "friendly layout name")
	description := fs.String("description", "", "longer layout description")
	var tags stringList
	fs.Var(&tags, "tag", "profile metadata, repeated as key=value")
	list := fs.Bool("list", false, "list profiles and layouts")
	migrate := fs.Bool("migrate", false, "import the old Bash configuration and built-in layouts")
	force := fs.Bool("force", false, "allow --migrate to replace an existing JSON config")
	configPath := fs.String("config", defaultConfigPath, "configuration file path")
	legacyPath := fs.String("legacy-config", defaultLegacyPath, "old layouts.conf path for --migrate")
	completionShell := fs.String("completion", "", "generate shell completion")
	completeKind := fs.String("complete", "", "print completion candidates (profiles or layouts)")
	help := fs.Bool("help", false, "show this help")
	shortHelp := fs.Bool("h", false, "show this help")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *help || *shortHelp {
		printUsage(stdout)
		return 0
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintf(stderr, "Error: unexpected argument %q\n\n", fs.Args()[0])
		printUsage(stderr)
		return 2
	}

	resolvedConfig := *configPath
	resolvedLegacy := *legacyPath
	if resolvedConfig == "" {
		resolvedConfig = configPathFromEnvironment()
	}
	if resolvedLegacy == "" {
		resolvedLegacy = filepath.Join(filepath.Dir(resolvedConfig), "layouts.conf")
	}

	if *completionShell != "" && *completeKind != "" {
		fmt.Fprintln(stderr, "Error: --completion and --complete cannot be used together")
		return 2
	}
	if *completionShell != "" {
		if *list || *saveID != "" || *applyID != "" || *profileID != "" || len(tags) != 0 || *migrate || *force || *name != "" || *description != "" {
			fmt.Fprintln(stderr, "Error: --completion cannot be combined with an action")
			return 2
		}
		script, err := completion.Generate(*completionShell)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 2
		}
		fmt.Fprint(stdout, script)
		return 0
	}
	if *completeKind != "" {
		if *list || *saveID != "" || *applyID != "" || len(tags) != 0 || *migrate || *force || *name != "" || *description != "" {
			fmt.Fprintln(stderr, "Error: --complete cannot be combined with an action")
			return 2
		}
		return completeCandidates(stdout, stderr, resolvedConfig, *completeKind, *profileID)
	}

	if *migrate {
		if *list || *saveID != "" || *applyID != "" || *profileID != "" || len(tags) != 0 || *name != "" || *description != "" {
			fmt.Fprintln(stderr, "Error: --migrate cannot be combined with profile, layout, tag, or list actions")
			return 2
		}
		return doMigrate(resolvedConfig, resolvedLegacy, *force, stdout, stderr)
	}
	if *saveID != "" && *applyID != "" {
		fmt.Fprintln(stderr, "Error: --save and --apply cannot be used together")
		return 2
	}
	if (*name != "" || *description != "") && *saveID == "" {
		fmt.Fprintln(stderr, "Error: --name and --description require --save")
		return 2
	}
	if (*saveID != "" || *applyID != "") && *profileID == "" {
		fmt.Fprintln(stderr, "Error: --profile is required with --save and --apply")
		return 2
	}
	if *list && (*saveID != "" || *applyID != "") {
		fmt.Fprintln(stderr, "Error: --list cannot be combined with --save or --apply")
		return 2
	}
	if len(tags) != 0 && *profileID == "" {
		fmt.Fprintln(stderr, "Error: --profile is required with --tag")
		return 2
	}
	if *list && len(tags) != 0 {
		fmt.Fprintln(stderr, "Error: --tag cannot be combined with --list")
		return 2
	}

	c, err := config.Load(resolvedConfig)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if *list {
		if err := printProfiles(stdout, c, *profileID); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}
	if len(tags) != 0 && *saveID == "" && *applyID == "" {
		p, err := config.FindProfile(&c, *profileID)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		if err := applyTags(p, tags); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 2
		}
		if err := config.Save(resolvedConfig, c); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Updated tags for profile %s\n", p.ID)
		return 0
	}

	raw, err := runner.List()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	snapshot, err := displayplacer.ParseList(raw)
	if err != nil {
		fmt.Fprintf(stderr, "Error: cannot parse displayplacer list: %v\n", err)
		return 1
	}
	discovered, err := profile.Discover(snapshot, c.LaptopIDs)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	if *saveID != "" {
		return saveLayout(stdout, stderr, resolvedConfig, c, *profileID, *saveID, *name, *description, tags, snapshot, discovered)
	}

	selected, err := chooseProfile(c, *profileID, discovered.External)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		if *profileID == "" {
			fmt.Fprintln(stderr, "Create or attach a profile with: toggle-display --profile NAME --save layout1")
		}
		return 1
	}

	var layout config.Layout
	known := true
	if *applyID != "" {
		var ok bool
		layout, ok = config.LayoutByID(selected, *applyID)
		if !ok {
			fmt.Fprintf(stderr, "Error: profile %q has no layout %q\n", selected.ID, *applyID)
			return 1
		}
	} else {
		layout, known, err = profile.NextLayout(selected, discovered.Laptop)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
	}
	argsToApply, err := profile.SubstituteArguments(layout.DisplayplacerArgs, discovered.Laptop, discovered.External)
	if err != nil {
		fmt.Fprintf(stderr, "Error: cannot apply layout %s: %v\n", layout.ID, err)
		return 1
	}
	if err := runner.Apply(argsToApply); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return displayplacer.ExitCode(err)
	}
	printSuccess(stdout, selected, layout, !known && *applyID == "")
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: toggle-display [options]

With no action, detect the connected monitor and toggle its next layout.
Profiles are selected automatically unless --profile is supplied.

Options:
  --profile NAME             select or create a monitor profile
  --save ID                  capture the current arrangement as a layout
  --name NAME                friendly name for a saved layout
  --description TEXT         description for a saved layout
  --apply ID                 apply one layout by id
  --tag KEY=VALUE            set profile metadata (may be repeated)
  --list                     list profiles, ids, tags, and layouts
  --migrate                  import old layouts.conf and built-in A-D layouts
  --force                    replace config when used with --migrate
  --config PATH              use an alternate config.json
  --legacy-config PATH       old layouts.conf path for --migrate
  --completion SHELL         print shell completion script (zsh)
  --complete KIND            print completion data (profiles or layouts)
  -h, --help                 show this help`)
}

func configPathFromEnvironment() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return config.ConfigPath(home, os.Getenv("XDG_CONFIG_HOME"))
}

func completeCandidates(stdout, stderr io.Writer, path, kind, requestedProfile string) int {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "Error: config %s does not exist; no completion candidates are available\n", path)
		return 1
	} else if err != nil {
		fmt.Fprintf(stderr, "Error: inspect config %s: %v\n", path, err)
		return 1
	}
	c, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	switch strings.ToLower(kind) {
	case "profiles":
		profiles := config.SortedProfiles(c.Profiles)
		for _, p := range profiles {
			fmt.Fprintln(stdout, p.ID)
		}
		return 0
	case "layouts":
		if requestedProfile == "" {
			fmt.Fprintln(stderr, "Error: --complete layouts requires --profile NAME")
			return 2
		}
		p, err := config.FindProfile(&c, requestedProfile)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		ids := make([]string, 0, len(p.Layouts))
		for _, layout := range p.Layouts {
			ids = append(ids, layout.ID)
		}
		sort.SliceStable(ids, func(i, j int) bool {
			if strings.EqualFold(ids[i], ids[j]) {
				return ids[i] < ids[j]
			}
			return strings.ToLower(ids[i]) < strings.ToLower(ids[j])
		})
		for _, id := range ids {
			fmt.Fprintln(stdout, id)
		}
		return 0
	default:
		fmt.Fprintf(stderr, "Error: unsupported completion kind %q (use profiles or layouts)\n", kind)
		return 2
	}
}

func chooseProfile(c config.Config, requested string, external displayplacer.Display) (config.Profile, error) {
	if requested != "" {
		p, err := config.FindProfile(&c, requested)
		if err != nil {
			return config.Profile{}, err
		}
		if !profile.Matches(*p, external) {
			return config.Profile{}, fmt.Errorf("connected monitor does not belong to profile %q (connected ids: %s)", p.ID, profile.ConnectedIDs(external))
		}
		return *p, nil
	}
	return profile.Select(c.Profiles, external)
}

func saveLayout(stdout, stderr io.Writer, path string, c config.Config, profileName, layoutID, layoutName, description string, tags []string, snapshot displayplacer.Snapshot, discovered profile.Discovery) int {
	if layoutID == "" {
		fmt.Fprintln(stderr, "Error: --save needs a layout id")
		return 2
	}
	profileIndex := -1
	for i := range c.Profiles {
		if strings.EqualFold(c.Profiles[i].ID, profileName) {
			profileIndex = i
			break
		}
	}
	if profileIndex < 0 {
		if err := validateProfileFlag(profileName); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 2
		}
		c.Profiles = append(c.Profiles, config.Profile{ID: profileName, Tags: map[string]string{}})
		profileIndex = len(c.Profiles) - 1
	}
	p := c.Profiles[profileIndex]
	if len(p.MonitorIDs) != 0 && !profile.Matches(p, discovered.External) {
		fmt.Fprintf(stderr, "Error: connected monitor does not belong to profile %q (connected ids: %s)\n", p.ID, profile.ConnectedIDs(discovered.External))
		return 1
	}
	ids, err := profile.MergeMonitorIDs(p.MonitorIDs, discovered.External)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	args, err := profile.CaptureArguments(snapshot, discovered.Laptop, discovered.External)
	if err != nil {
		fmt.Fprintf(stderr, "Error: cannot save layout: %v\n", err)
		return 1
	}
	layout := config.Layout{
		ID: layoutID, Name: layoutName, Description: description,
		LaptopOrigin: discovered.Laptop.Origin.String(), DisplayplacerArgs: args,
	}
	if layout.Name == "" {
		layout.Name = layoutID
	}
	if existing, ok := config.LayoutByID(p, layoutID); ok {
		layout.Name = existing.Name
		layout.Description = existing.Description
		if layoutName != "" {
			layout.Name = layoutName
		}
		if description != "" {
			layout.Description = description
		}
		for i := range p.Layouts {
			if strings.EqualFold(p.Layouts[i].ID, layoutID) {
				p.Layouts[i] = layout
			}
		}
	} else {
		p.Layouts = append(p.Layouts, layout)
		p.LayoutOrder = append(p.LayoutOrder, layoutID)
	}
	p.MonitorIDs = ids
	if p.Tags == nil {
		p.Tags = map[string]string{}
	}
	if err := applyTags(&p, tags); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	c.Profiles[profileIndex] = p
	if err := config.Save(path, c); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Saved %s -> %s (%s)\n", p.ID, layout.ID, layout.Name)
	return 0
}

func validateProfileFlag(value string) error {
	// Config.Validate supplies the canonical wording, while this small helper
	// avoids writing a partially-created profile before finding a bad id.
	c := config.Default()
	c.Profiles = []config.Profile{{ID: value, MonitorIDs: []identity.Identifier{{Type: identity.Persistent, Value: "temporary"}}}}
	if err := c.Validate(); err != nil {
		return err
	}
	return nil
}

func applyTags(p *config.Profile, values []string) error {
	if p.Tags == nil {
		p.Tags = map[string]string{}
	}
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return fmt.Errorf("invalid tag %q; use KEY=VALUE", value)
		}
		key := strings.TrimSpace(parts[0])
		if strings.ContainsAny(key, " \t\r\n") {
			return fmt.Errorf("invalid tag key %q", key)
		}
		p.Tags[key] = parts[1]
	}
	return nil
}

func printSuccess(w io.Writer, p config.Profile, layout config.Layout, unknown bool) {
	fmt.Fprintf(w, "%s -> %s (%s)", p.ID, layout.ID, layout.Name)
	if layout.Description != "" {
		fmt.Fprintf(w, ": %s", layout.Description)
	}
	if unknown {
		fmt.Fprint(w, " [current layout unknown; applied first layout]")
	}
	fmt.Fprintln(w)
}

func printProfiles(w io.Writer, c config.Config, requested string) error {
	profiles := c.Profiles
	if requested != "" {
		p, err := config.FindProfile(&c, requested)
		if err != nil {
			return err
		}
		profiles = []config.Profile{*p}
	}
	if len(profiles) == 0 {
		fmt.Fprintln(w, "No profiles configured.")
		return nil
	}
	for _, p := range config.SortedProfiles(profiles) {
		fmt.Fprintf(w, "Profile: %s\n", p.ID)
		if len(p.Tags) == 0 {
			fmt.Fprintln(w, "  Tags: (none)")
		} else {
			keys := make([]string, 0, len(p.Tags))
			for key := range p.Tags {
				keys = append(keys, key)
			}
			// Stable output makes --list useful in scripts and reviews.
			for i := 0; i < len(keys); i++ {
				for j := i + 1; j < len(keys); j++ {
					if strings.ToLower(keys[j]) < strings.ToLower(keys[i]) {
						keys[i], keys[j] = keys[j], keys[i]
					}
				}
			}
			for i, key := range keys {
				if i == 0 {
					fmt.Fprintf(w, "  Tags: %s=%s\n", key, p.Tags[key])
				} else {
					fmt.Fprintf(w, "        %s=%s\n", key, p.Tags[key])
				}
			}
		}
		fmt.Fprintln(w, "  Monitor IDs:")
		for _, id := range p.MonitorIDs {
			fmt.Fprintf(w, "    %s\n", id)
		}
		fmt.Fprintln(w, "  Layouts:")
		if len(p.LayoutOrder) == 0 {
			fmt.Fprintln(w, "    (none)")
		}
		for _, id := range p.LayoutOrder {
			layout, ok := config.LayoutByID(p, id)
			if !ok {
				return fmt.Errorf("profile %q has invalid layout order entry %q", p.ID, id)
			}
			fmt.Fprintf(w, "    %s: %s\n", layout.ID, layout.Name)
			if layout.Description != "" {
				fmt.Fprintf(w, "      %s\n", layout.Description)
			}
		}
	}
	return nil
}

func doMigrate(path, legacyPath string, force bool, stdout, stderr io.Writer) int {
	if info, err := os.Stat(path); err == nil && !force {
		fmt.Fprintf(stderr, "Error: %s already exists; use --force only after backing it up\n", path)
		return 1
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "Error: inspect config: %v\n", err)
		return 1
	} else if info != nil && force {
		if backup, err := backupFile(path); err != nil {
			fmt.Fprintf(stderr, "Error: back up existing config: %v\n", err)
			return 1
		} else {
			fmt.Fprintf(stdout, "Backed up existing config to %s\n", backup)
		}
	}
	if _, err := os.Stat(legacyPath); err == nil {
		backup, err := backupFile(legacyPath)
		if err != nil {
			fmt.Fprintf(stderr, "Error: back up legacy config: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Backed up legacy config to %s\n", backup)
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "Error: inspect legacy config: %v\n", err)
		return 1
	}
	base := migration.BuiltinConfig()
	c, err := migration.ImportLegacy(base, legacyPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if err := config.Save(path, c); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Migrated display profiles to %s\n", path)
	return 0
}

func backupFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	candidate := path + ".bak"
	for i := 1; ; i++ {
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			break
		}
		candidate = fmt.Sprintf("%s.bak.%d", path, i)
	}
	if err := os.WriteFile(candidate, data, 0o600); err != nil {
		return "", err
	}
	return candidate, nil
}
