# toggle-display

A small macOS command-line tool for cycling display arrangements with
[`displayplacer`](https://github.com/jakehilborn/displayplacer). The Go program
identifies the connected physical monitor, selects its profile, detects the
current arrangement, and applies the next saved layout.

## Requirements

- macOS
- Go (to build the tool)
- `displayplacer` (`brew install displayplacer`)

## Build and install

From this directory:

```sh
make build                         # writes ./toggle-display
make install                       # writes ~/.local/bin/toggle-display
```

`toggle-display.sh` remains a compatible entry point. It runs a built binary
beside the wrapper when one exists, and otherwise builds a short-lived Go
binary, so existing aliases can continue to point at the script while developing.

## Daily use

```sh
toggle-display
```

The no-argument command queries `displayplacer list` once, requires exactly one
laptop and one external display, matches the external display to exactly one
profile, and cycles that profile's explicit layout order. An unknown current
arrangement applies the first layout. A successful result looks like:

```text
hp_home -> layout2 (under-monitor): External monitor above the laptop
```

A setup with zero or multiple external displays, an unknown monitor, or an
ambiguous profile is refused rather than guessed.

## Profiles and layouts

Save the current arrangement without changing it:

```sh
toggle-display --profile hp_home \
  --save layout1 \
  --name next-to-laptop \
  --description "External monitor on the left, laptop on the right"

# Rearrange the displays, then:
toggle-display --profile hp_home \
  --save layout2 \
  --name under-monitor \
  --description "External monitor above the laptop"
```

The first save creates the profile and records the useful screen identifiers.
Saving an existing layout id updates its captured arrangement. Profile and
layout ids use letters, numbers, `.`, `_`, and `-`; friendly names and
descriptions may contain spaces and punctuation.

Other operations:

```sh
toggle-display --profile hp_home                 # explicit-profile toggle
toggle-display --profile hp_home --apply layout1
toggle-display --list
toggle-display --profile hp_home --list
toggle-display --profile hp_home --tag place=home --tag model="HP 27f 4k"
```

Tags are descriptive only and are never used to identify hardware. An explicit
profile is still verified against the connected monitor before applying it.

## Configuration and migration

The versioned JSON file is stored at:

```text
~/.config/toggle-display/config.json
```

`XDG_CONFIG_HOME` and `--config PATH` are supported. Layout commands are stored
as individual JSON arguments with `LAPTOP_ID` and `EXTERNAL_ID` placeholders.
They are passed directly to `exec.Command`; configuration is never sourced as
Bash and saved commands are never evaluated by a shell. Writes use a synced
temporary file and an atomic rename.

To import the old hardcoded HP/DELL layouts and any old monitor-specific
`layouts.conf` entries:

```sh
toggle-display --migrate
```

Migration is conservative: it will not replace an existing JSON file unless
`--force` is supplied, and creates a `.bak` backup of the old Bash config (and
of an overwritten JSON config). The imported A-D layouts remain available as a
compatibility aid; new profiles can use ids such as `layout1`.

## Development

```sh
make test
go test ./...
```

Parser fixtures are checked in under `testdata/displayplacer`. Domain logic is
split into `internal/config`, `internal/displayplacer`, `internal/profile`, and
`internal/migration` so tests can inject displayplacer output and an apply
runner without physical monitors.
