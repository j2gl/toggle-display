// Package identity contains screen identifiers shared by configuration and
// displayplacer discovery.
package identity

import (
	"fmt"
	"strings"
)

// Type describes the source of a screen identifier.
type Type string

const (
	Persistent Type = "persistent"
	Serial     Type = "serial"
)

// Identifier is a displayplacer screen identifier.
type Identifier struct {
	Type  Type   `json:"type"`
	Value string `json:"value"`
}

func (i Identifier) String() string {
	if i.Type == "" {
		return i.Value
	}
	return fmt.Sprintf("%s=%s", i.Type, i.Value)
}

// Normalize makes comparisons insensitive to accidental case and whitespace.
// Screen ids themselves are conventionally uppercase UUIDs, but displayplacer
// identifiers are case-insensitive in practice.
func Normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// Key is suitable for comparing identifiers from different sources.
func (i Identifier) Key() string {
	return string(i.Type) + ":" + Normalize(i.Value)
}

// IsGenericSerial reports serial values which are commonly EDID placeholders,
// not hardware identities. They must not be the only identity of a profile.
func IsGenericSerial(value string) bool {
	v := Normalize(value)
	// s0 is used when no serial is available. 0x01010101 is emitted by a
	// number of displays as an unset EDID serial (16843009 decimal).
	return v == "s0" || v == "s16843009" || v == "s4294967295"
}

// Useful reports whether an identifier can safely be used as a profile key.
func (i Identifier) Useful() bool {
	if Normalize(i.Value) == "" {
		return false
	}
	return i.Type != Serial || !IsGenericSerial(i.Value)
}

// Ordered returns identifiers in the matching priority order, dropping empty
// and generic values and duplicate entries.
func Ordered(ids ...Identifier) []Identifier {
	seen := make(map[string]bool)
	result := make([]Identifier, 0, len(ids))
	for _, preferredType := range []Type{Serial, Persistent} {
		for _, id := range ids {
			if id.Type != preferredType || !id.Useful() || seen[id.Key()] {
				continue
			}
			seen[id.Key()] = true
			result = append(result, id)
		}
	}
	return result
}
