package displayplacer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "displayplacer", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestParseFixtures(t *testing.T) {
	for _, test := range []struct {
		file     string
		serial   string
		external string
		laptopY  int
		refresh  float64
		scaling  string
	}{
		{file: "hp.txt", serial: "s0", external: "6F9FB1D9-2284-44F6-8357-9B84666EEBD5", laptopY: 458, refresh: 60, scaling: "on"},
		{file: "dell.txt", serial: "s1093808706", external: "4BBE0CEB-FD34-4B58-AA4B-B701217F35EA", laptopY: 0, refresh: 144, scaling: "off"},
	} {
		snapshot, err := ParseList(fixture(t, test.file))
		if err != nil {
			t.Fatalf("%s: %v", test.file, err)
		}
		if len(snapshot.Displays) != 2 || len(snapshot.Arrangement) != 2 {
			t.Fatalf("%s: got %d displays and %d arrangement args", test.file, len(snapshot.Displays), len(snapshot.Arrangement))
		}
		if snapshot.Displays[0].PersistentID != test.external || snapshot.Displays[0].SerialID != test.serial {
			t.Fatalf("%s: identifiers were not parsed: %+v", test.file, snapshot.Displays[0])
		}
		if snapshot.Displays[0].RefreshRate != test.refresh || snapshot.Displays[0].Scaling != test.scaling {
			t.Fatalf("%s: mode fields were not parsed: %+v", test.file, snapshot.Displays[0])
		}
		if snapshot.Displays[0].Spec.ID != test.external {
			t.Fatalf("%s: arrangement was not associated with display", test.file)
		}
		if snapshot.Displays[1].Origin.Y != test.laptopY {
			t.Fatalf("%s: origin was not parsed: %+v", test.file, snapshot.Displays[1].Origin)
		}
	}
}

func TestDisconnectedAndUnknownFixtures(t *testing.T) {
	disconnected, err := ParseList(fixture(t, "disconnected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(disconnected.Displays) != 1 {
		t.Fatalf("got %d displays", len(disconnected.Displays))
	}
	unknown, err := ParseList(fixture(t, "unknown.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Displays[0].Rotation != 90 || unknown.Displays[0].Spec.Rotation != 90 {
		t.Fatalf("rotation was not parsed: %+v", unknown.Displays[0])
	}
}

func TestParseAndReplaceArguments(t *testing.T) {
	line := `displayplacer "id:EXTERNAL_ID res:1920x1080 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0" "id:LAPTOP_ID res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(1,2) degree:0"`
	args, err := ParseArrangementCommand(line)
	if err != nil || len(args) != 2 {
		t.Fatalf("parse command: %v %#v", err, args)
	}
	spec, err := ParseSpec(args[1])
	if err != nil || spec.Origin.X != 1 || spec.Origin.Y != 2 {
		t.Fatalf("parse spec: %v %+v", err, spec)
	}
	replaced, err := ReplaceSpecID(args[0], "actual-id")
	if err != nil || !strings.Contains(replaced, "id:actual-id") {
		t.Fatalf("replace id: %v %s", err, replaced)
	}
}
