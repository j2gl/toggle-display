package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"toggle-display/internal/displayplacer"
)

type fakeRunner struct {
	output    string
	listCalls int
	applied   []string
	applyErr  error
}

func (f *fakeRunner) List() (string, error) {
	f.listCalls++
	return f.output, nil
}
func (f *fakeRunner) Apply(args []string) error {
	f.applied = append([]string(nil), args...)
	return f.applyErr
}

func mainFixture(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "displayplacer", "hp.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSaveAndAutomaticToggle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	layoutOneState := strings.ReplaceAll(mainFixture(t), "(2560,458)", "(542,1440)")
	layoutTwoState := mainFixture(t)
	fake := &fakeRunner{output: layoutOneState}
	var out, errOut bytes.Buffer
	if status := run([]string{"--config", path, "--profile", "hp_home", "--save", "layout1", "--name", "next-to-laptop"}, &out, &errOut, fake, "", ""); status != 0 {
		t.Fatalf("save layout1: status %d out=%s err=%s", status, out.String(), errOut.String())
	}
	fake.output = layoutTwoState
	out.Reset()
	errOut.Reset()
	if status := run([]string{"--config", path, "--profile", "hp_home", "--save", "layout2", "--name", "under-monitor", "--description", "above the laptop"}, &out, &errOut, fake, "", ""); status != 0 {
		t.Fatalf("save layout2: status %d out=%s err=%s", status, out.String(), errOut.String())
	}
	// The state is layout1, so automatic matching should select hp_home and
	// apply layout2 with two complete, shell-free arguments.
	fake.output = layoutOneState
	fake.applied = nil
	out.Reset()
	errOut.Reset()
	if status := run([]string{"--config", path}, &out, &errOut, fake, "", ""); status != 0 {
		t.Fatalf("toggle: status %d out=%s err=%s", status, out.String(), errOut.String())
	}
	if fake.listCalls != 3 || len(fake.applied) != 2 || !strings.Contains(out.String(), "hp_home -> layout2 (under-monitor): above the laptop") {
		t.Fatalf("unexpected toggle: calls=%d args=%#v out=%s", fake.listCalls, fake.applied, out.String())
	}
	if strings.Contains(strings.Join(fake.applied, " "), "EXTERNAL_ID") || strings.Contains(strings.Join(fake.applied, " "), "LAPTOP_ID") {
		t.Fatal("role placeholders reached the runner")
	}
}

func TestFailedApplyDoesNotPrintSuccess(t *testing.T) {
	fake := &fakeRunner{output: mainFixture(t), applyErr: errors.New("displayplacer failed")}
	var out, errOut bytes.Buffer
	// Save two layouts first so toggle can run without a state-file shortcut.
	path := filepath.Join(t.TempDir(), "config.json")
	if status := run([]string{"--config", path, "--profile", "hp_home", "--save", "one"}, ioDiscard{}, ioDiscard{}, fake, "", ""); status != 0 {
		t.Fatalf("initial save: %d", status)
	}
	fake.output = strings.ReplaceAll(mainFixture(t), "(2560,458)", "(542,1440)")
	fake.applyErr = nil
	if status := run([]string{"--config", path, "--profile", "hp_home", "--save", "two"}, ioDiscard{}, ioDiscard{}, fake, "", ""); status != 0 {
		t.Fatalf("second save: %d", status)
	}
	fake.output = mainFixture(t)
	fake.applyErr = errors.New("failed")
	status := run([]string{"--config", path, "--profile", "hp_home"}, &out, &errOut, fake, "", "")
	if status == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "failed") {
		t.Fatalf("failed apply was reported as success: status=%d out=%q err=%q", status, out.String(), errOut.String())
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

var _ displayplacer.Runner = (*fakeRunner)(nil)
