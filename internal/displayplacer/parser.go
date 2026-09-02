// Package displayplacer parses displayplacer output and provides a shell-free
// command runner for applying saved screen specifications.
package displayplacer

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"toggle-display/internal/identity"
)

type Point struct {
	X int
	Y int
}

func (p Point) String() string { return fmt.Sprintf("(%d,%d)", p.X, p.Y) }

// ScreenSpec is one argument accepted by displayplacer. Raw is retained so a
// save operation can preserve fields displayplacer adds in future versions.
type ScreenSpec struct {
	ID          string
	Resolution  string
	RefreshRate float64
	ColorDepth  int
	Enabled     bool
	Scaling     string
	Origin      Point
	Rotation    int
	Raw         string
}

// Display is a screen record from displayplacer list.
type Display struct {
	PersistentID string
	SerialID     string
	ContextualID string
	Type         string
	Name         string
	Resolution   string
	RefreshRate  float64
	ColorDepth   int
	Scaling      string
	Origin       Point
	Rotation     int
	Enabled      bool
	EnabledSet   bool
	Spec         ScreenSpec
}

func (d Display) Identifiers() []identity.Identifier {
	return identity.Ordered(
		identity.Identifier{Type: identity.Serial, Value: d.SerialID},
		identity.Identifier{Type: identity.Persistent, Value: d.PersistentID},
		identity.Identifier{Type: identity.Contextual, Value: d.ContextualID},
	)
}

func (d Display) AllIdentifiers() []identity.Identifier {
	return []identity.Identifier{
		{Type: identity.Persistent, Value: d.PersistentID},
		{Type: identity.Serial, Value: d.SerialID},
		{Type: identity.Contextual, Value: d.ContextualID},
	}
}

func (d Display) HasIdentifier(id identity.Identifier) bool {
	for _, observed := range d.AllIdentifiers() {
		if observed.Type == id.Type && identity.Normalize(observed.Value) == identity.Normalize(id.Value) {
			return true
		}
	}
	return false
}

// PreferredID is an identifier suitable for an apply command. Real serials
// are preferred because they survive persistent-id churn; placeholders are
// skipped.
func (d Display) PreferredID() string {
	if d.SerialID != "" && !identity.IsGenericSerial(d.SerialID) {
		return d.SerialID
	}
	if d.PersistentID != "" {
		return d.PersistentID
	}
	if d.ContextualID != "" {
		return d.ContextualID
	}
	return d.SerialID
}

type Snapshot struct {
	Displays    []Display
	Arrangement []ScreenSpec
}

var (
	originRE = regexp.MustCompile(`\(\s*(-?[0-9]+)\s*,\s*(-?[0-9]+)\s*\)`)
	fieldRE  = map[string]*regexp.Regexp{
		"id":          regexp.MustCompile(`(?:^|\s)id:([^\s]+)`),
		"res":         regexp.MustCompile(`(?:^|\s)res:([^\s]+)`),
		"hz":          regexp.MustCompile(`(?:^|\s)hz:([^\s]+)`),
		"color_depth": regexp.MustCompile(`(?:^|\s)color_depth:([^\s]+)`),
		"enabled":     regexp.MustCompile(`(?:^|\s)enabled:([^\s]+)`),
		"scaling":     regexp.MustCompile(`(?:^|\s)scaling:([^\s]+)`),
		"origin":      regexp.MustCompile(`(?:^|\s)origin:\(\s*(-?[0-9]+)\s*,\s*(-?[0-9]+)\s*\)`),
		"degree":      regexp.MustCompile(`(?:^|\s)(?:degree|rotation):(-?[0-9]+)`),
	}
)

// ParseList parses one captured output of displayplacer list. It deliberately
// ignores mode-list lines and unknown informational lines.
func ParseList(output string) (Snapshot, error) {
	var snapshot Snapshot
	var current *Display
	var arrangement []ScreenSpec
	flush := func() {
		if current != nil {
			current.Spec = ScreenSpec{
				ID:          current.PersistentID,
				Resolution:  current.Resolution,
				RefreshRate: current.RefreshRate,
				ColorDepth:  current.ColorDepth,
				Enabled:     current.Enabled,
				Scaling:     current.Scaling,
				Origin:      current.Origin,
				Rotation:    current.Rotation,
			}
			snapshot.Displays = append(snapshot.Displays, *current)
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	// A future displayplacer version could print a very long mode list.
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "persistent screen id:"):
			flush()
			value := strings.TrimSpace(line[strings.Index(line, ":")+1:])
			current = &Display{PersistentID: firstWord(value)}
		case current != nil && strings.HasPrefix(lower, "contextual screen id:"):
			current.ContextualID = firstWord(afterColon(line))
		case current != nil && strings.HasPrefix(lower, "serial screen id:"):
			current.SerialID = firstWord(afterColon(line))
		case current != nil && strings.HasPrefix(lower, "type:"):
			current.Type = strings.TrimSpace(afterColon(line))
			if current.Name == "" {
				current.Name = current.Type
			}
		case current != nil && strings.HasPrefix(lower, "name:"):
			current.Name = strings.TrimSpace(afterColon(line))
		case current != nil && strings.HasPrefix(lower, "resolution:"):
			current.Resolution = firstWord(afterColon(line))
		case current != nil && (strings.HasPrefix(lower, "hertz:") || strings.HasPrefix(lower, "refresh rate:")):
			current.RefreshRate, _ = strconv.ParseFloat(firstWord(afterColon(line)), 64)
		case current != nil && strings.HasPrefix(lower, "color depth:"):
			current.ColorDepth, _ = strconv.Atoi(firstWord(afterColon(line)))
		case current != nil && strings.HasPrefix(lower, "scaling:"):
			current.Scaling = firstWord(afterColon(line))
		case current != nil && strings.HasPrefix(lower, "origin:"):
			if matches := originRE.FindStringSubmatch(line); len(matches) == 3 {
				current.Origin.X, _ = strconv.Atoi(matches[1])
				current.Origin.Y, _ = strconv.Atoi(matches[2])
			}
		case current != nil && (strings.HasPrefix(lower, "rotation:") || strings.HasPrefix(lower, "degree:")):
			current.Rotation, _ = strconv.Atoi(firstWord(afterColon(line)))
		case current != nil && strings.HasPrefix(lower, "enabled:"):
			current.Enabled, _ = strconv.ParseBool(firstWord(afterColon(line)))
			current.EnabledSet = true
		}

		if strings.HasPrefix(lower, "displayplacer ") && strings.Contains(lower, "id:") {
			args, err := ParseArrangementCommand(line)
			if err != nil {
				return Snapshot{}, err
			}
			parsedArrangement := make([]ScreenSpec, 0, len(args))
			for _, arg := range args {
				spec, err := ParseSpec(arg)
				if err != nil {
					return Snapshot{}, fmt.Errorf("parse current arrangement: %w", err)
				}
				parsedArrangement = append(parsedArrangement, spec)
			}
			// The last arrangement command is the current one if a future
			// displayplacer version prints more than one informational command.
			arrangement = parsedArrangement
		}
	}
	if err := scanner.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("read displayplacer output: %w", err)
	}
	flush()
	if len(snapshot.Displays) == 0 {
		return Snapshot{}, errors.New("displayplacer list contained no screens")
	}
	if len(arrangement) > 0 {
		snapshot.Arrangement = arrangement
		for i := range snapshot.Displays {
			for _, spec := range arrangement {
				if displayHasValue(snapshot.Displays[i], spec.ID) {
					snapshot.Displays[i].Spec = spec
					break
				}
			}
		}
	}
	return snapshot, nil
}

// Parse is a short alias useful to callers and tests.
func Parse(output string) (Snapshot, error) { return ParseList(output) }

func displayHasValue(display Display, value string) bool {
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

func firstWord(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func afterColon(value string) string {
	if i := strings.IndexByte(value, ':'); i >= 0 {
		return strings.TrimSpace(value[i+1:])
	}
	return ""
}

// ParseSpec parses a single displayplacer screen argument.
func ParseSpec(raw string) (ScreenSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ScreenSpec{}, errors.New("empty displayplacer screen specification")
	}
	get := func(name string) string {
		match := fieldRE[name].FindStringSubmatch(raw)
		if len(match) < 2 {
			return ""
		}
		return match[1]
	}
	spec := ScreenSpec{Raw: raw, ID: get("id"), Resolution: get("res"), Scaling: get("scaling")}
	if spec.ID == "" {
		return ScreenSpec{}, fmt.Errorf("displayplacer screen specification has no id: %q", raw)
	}
	if value := get("hz"); value != "" {
		spec.RefreshRate, _ = strconv.ParseFloat(value, 64)
	}
	if value := get("color_depth"); value != "" {
		spec.ColorDepth, _ = strconv.Atoi(value)
	}
	if value := get("enabled"); value != "" {
		spec.Enabled, _ = strconv.ParseBool(value)
	}
	if matches := fieldRE["origin"].FindStringSubmatch(raw); len(matches) == 3 {
		spec.Origin.X, _ = strconv.Atoi(matches[1])
		spec.Origin.Y, _ = strconv.Atoi(matches[2])
	}
	if value := get("degree"); value != "" {
		spec.Rotation, _ = strconv.Atoi(value)
	}
	return spec, nil
}

// ParseArrangementCommand extracts quoted screen arguments from the command
// printed by displayplacer list. It is a small parser rather than a shell
// invocation, so saved values can never become shell code.
func ParseArrangementCommand(line string) ([]string, error) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(strings.ToLower(line), "displayplacer ") {
		return nil, fmt.Errorf("not a displayplacer arrangement command: %q", line)
	}
	text := strings.TrimSpace(line[len("displayplacer "):])
	var args []string
	for i := 0; i < len(text); {
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}
		if i == len(text) {
			break
		}
		var b bytes.Buffer
		quoted := false
		for i < len(text) {
			ch := text[i]
			if quoted {
				switch ch {
				case '"':
					quoted = false
					i++
				case '\\':
					if i+1 >= len(text) {
						return nil, errors.New("unterminated escape in displayplacer command")
					}
					b.WriteByte(text[i+1])
					i += 2
				default:
					b.WriteByte(ch)
					i++
				}
				continue
			}
			switch ch {
			case '"':
				quoted = true
				i++
			case ' ', '\t':
				i++
				if b.Len() > 0 {
					goto argumentDone
				}
			case '\\':
				if i+1 >= len(text) {
					return nil, errors.New("unterminated escape in displayplacer command")
				}
				b.WriteByte(text[i+1])
				i += 2
			default:
				b.WriteByte(ch)
				i++
			}
		}
		if quoted {
			return nil, errors.New("unterminated quote in displayplacer command")
		}
	argumentDone:
		if b.Len() > 0 {
			args = append(args, b.String())
		}
	}
	if len(args) == 0 {
		return nil, errors.New("displayplacer arrangement command has no arguments")
	}
	return args, nil
}

// ReplaceSpecID replaces only the id field of a displayplacer argument.
func ReplaceSpecID(raw, id string) (string, error) {
	if _, err := ParseSpec(raw); err != nil {
		return "", err
	}
	valueLoc := fieldRE["id"].FindStringSubmatchIndex(raw)
	if len(valueLoc) < 4 {
		return "", fmt.Errorf("displayplacer screen specification has an invalid id: %q", raw)
	}
	return raw[:valueLoc[2]] + id + raw[valueLoc[3]:], nil
}

// CommandError retains displayplacer's stderr and exit status for a useful CLI
// error without ever treating it as a successful layout change.
type CommandError struct {
	Operation string
	Stderr    string
	Err       error
}

func (e *CommandError) Error() string {
	message := e.Operation + " failed"
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	if stderr := strings.TrimSpace(e.Stderr); stderr != "" {
		message += ": " + stderr
	}
	return message
}

func (e *CommandError) Unwrap() error { return e.Err }

// ExitCode returns a subprocess exit status when one is available. It is
// intentionally kept separate from CommandError so injected test runners can
// return ordinary errors.
func ExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
		return exitErr.ExitCode()
	}
	return 1
}

type Runner interface {
	List() (string, error)
	Apply(args []string) error
}

type CommandRunner struct {
	Binary string
}

func NewCommandRunner(binary string) CommandRunner {
	if binary == "" {
		binary = "displayplacer"
	}
	return CommandRunner{Binary: binary}
}

func (r CommandRunner) List() (string, error) {
	cmd := exec.Command(r.Binary, "list")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &CommandError{Operation: "displayplacer list", Stderr: stderr.String(), Err: err}
	}
	return stdout.String(), nil
}

func (r CommandRunner) Apply(args []string) error {
	cmd := exec.Command(r.Binary, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return &CommandError{Operation: "displayplacer apply", Stderr: stderr.String(), Err: err}
	}
	return nil
}
