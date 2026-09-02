package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportLegacyWithoutSourcing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layouts.conf")
	marker := filepath.Join(t.TempDir(), "should-not-exist")
	legacy := fmt.Sprintf(`# HP monitor persistent id: monitor-legacy
LAYOUT_custom_MONITOR_monitor_legacy_TEMPLATE='displayplacer "id:EXTERNAL_ID res:1x1" "id:LAPTOP_ID res:1x1"'
LAYOUT_custom_MONITOR_monitor_legacy_ORIGIN='(12,34)'
LAYOUT_custom_MONITOR_monitor_legacy_DESC='friendly layout with spaces'
# This must remain data, not execute.
$(touch %s)
`, marker)
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := ImportLegacy(BuiltinConfig(), path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range c.Profiles {
		for _, layout := range p.Layouts {
			if layout.ID == "custom" && strings.Contains(layout.Description, "spaces") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("legacy layout was not imported")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("legacy content was executed")
	}
}
