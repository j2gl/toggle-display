# toggle-display: Profile and Layout Design

> Implementation note: the Go migration adopts the versioned JSON format in
> `doc/go-migration-plan.md` at `~/.config/toggle-display/config.json`. The
> directory/INI layout below is the original design sketch and is retained for
> its domain vocabulary; use the Go CLI and README for current storage details.

## Goal

Make the configuration match the way displays are used:

- A **monitor profile** represents one physical monitor and has a friendly name.
- A **layout** represents one complete display arrangement for that monitor.
- A layout has a short stable id, a short friendly name, and an optional longer
  description.
- The monitor's screen ids are used for automatic profile selection; names and
  tags are for people.

This removes the current HP/DELL and A/B assumptions from the user-facing model.

## Example model

```text
Profile: hp_home
Tags: place=home, manufacturer=HP
Monitor ids: persistent-id, serial-id, ...

Layouts:
  layout1
    name: next-to-laptop
    description: External monitor on the left, laptop on the right

  layout2
    name: under-monitor
    description: External monitor above the laptop for meetings
```

`hp_home` is the profile id. `layout1` and `layout2` are layout ids used on the
command line. The names and descriptions can contain spaces and punctuation.

The `place` tag is descriptive metadata. It must not be used for automatic
matching because the script cannot determine whether a monitor is physically at
home or in the office. The monitor's hardware ids are the source of truth.

## Proposed commands

### Create a profile and save its first layout

The first save attaches the currently connected external monitor's known ids to
the named profile. The profile is created automatically if it does not exist.

```sh
./toggle-display.sh \
  --profile hp_home \
  --save layout1 \
  --name next-to-laptop \
  --description "External monitor on the left, laptop on the right"
```

The current arrangement is captured; the command does not change the displays.
The monitor profile should store every useful id reported at this point, while
preferring a real serial id and retaining the persistent id as a fallback. A
placeholder serial such as `s0` must not be treated as a unique hardware id.

### Save another layout

Rearrange the displays, then save the second arrangement:

```sh
./toggle-display.sh \
  --profile hp_home \
  --save layout2 \
  --name under-monitor \
  --description "External monitor above the laptop for meetings"
```

Saving an existing layout id updates that layout for the selected profile.

`--name` and `--description` should be optional. If omitted, the layout id is
used as the display name and the description is empty.

### Toggle a profile

```sh
./toggle-display.sh --profile hp_home
```

The script reads the current laptop origin, finds the matching layout, and
applies the next layout in the profile's saved order. With two layouts this is
`layout1 -> layout2 -> layout1`. If the current arrangement is not recognized,
the first layout is applied.

Output should identify all relevant names without requiring long arguments:

```text
hp_home -> layout2 (under-monitor): External monitor above the laptop for meetings
```

A profile with zero or one layout cannot be toggled and should produce a useful
error explaining that another layout must be saved.

### Apply a specific layout

```sh
./toggle-display.sh --profile hp_home --apply layout1
```

The layout id is deliberately short; the friendly name and description are
shown in status output.

### Automatic profile selection

With no explicit profile, the script finds the connected external monitor by
matching its observed ids against registered profiles:

```sh
./toggle-display.sh
```

If exactly one profile matches, it is toggled. The output includes the selected
profile and layout name. If no profile matches, the script prints the connected
ids and explains how to create or attach a profile. If multiple profiles match
(for example, because a weak/non-unique id was registered), it refuses to guess
and asks for `--profile`.

An explicit `--profile NAME` must still verify that the selected profile matches
the connected monitor. This prevents accidentally applying a home profile to an
unrelated office display. A deliberate `--rebind` operation can be added later
for replacing a monitor or repairing ids.

### Inspect profiles

```sh
./toggle-display.sh --list
./toggle-display.sh --profile hp_home --list
```

The list should show profile ids, tags, matched monitor ids, and each layout's
id/name/description. This makes the configuration understandable without
opening the config files.

## Storage design

The implementation stores one versioned JSON document rather than constructing
Bash variable names from user input:

```text
~/.config/toggle-display/config.json
```

Its essential shape is:

```json
{
  "version": 1,
  "laptop_ids": [{"type": "serial", "value": "s4251086178"}],
  "profiles": [{
    "id": "hp_home",
    "tags": {"place": "home"},
    "monitor_ids": [{"type": "persistent", "value": "..."}],
    "layout_order": ["layout1", "layout2"],
    "layouts": [{
      "id": "layout1",
      "name": "next-to-laptop",
      "description": "External monitor on the left",
      "laptop_origin": "(2560,458)",
      "displayplacer_args": ["id:EXTERNAL_ID ...", "id:LAPTOP_ID ..."]
    }]
  }]
}
```

Each displayplacer screen specification is an argument, not an executable
string. `LAPTOP_ID` and `EXTERNAL_ID` are substituted in memory and passed
without a shell. Profile/layout ids are validated; friendly names, tags, and
descriptions may contain spaces and punctuation. The layout order is explicit
and never inferred from filesystem ordering. Writes are atomic.

## Matching and safety rules

1. Discover the current display list once per invocation.
2. Match profiles using all registered ids, preferring serial ids when they are
   real and unique, then persistent ids.
3. Never register a generic placeholder such as `s0` as the only identity.
4. Require a profile match before applying a profile in automatic mode.
5. Require at least one laptop and one external screen before saving or applying.
6. Detect the active layout from the current arrangement, not from a state file.
7. Check and return the exit status from `displayplacer`; do not print success
   when the arrangement failed.
8. Print the profile id, layout id, friendly name, and description after a
   successful change.

## Migration from the current implementation

- Existing built-in `A`/`B` and `C`/`D` layouts remain as temporary fallback
  layouts during migration.
- Existing saved global layouts can be imported into a selected profile.
- `HP_IDS` and `DELL_IDS` should not remain the primary configuration mechanism;
  their values should be copied into profile id lists when a profile is first
  attached.
- Existing invalid names such as `hp-juanjo` should be imported as normal
  friendly profile/layout ids because the new storage format does not depend on
  Bash variable names.
- Once profiles are migrated, the old type-specific arrays and A-D terminology
  can be removed from the user-facing help and documentation.

## Summary

The intended mental model is:

```text
connected monitor id -> monitor profile -> ordered layouts -> displayplacer command
```

Typical daily usage becomes:

```sh
./toggle-display.sh                  # detect monitor and toggle its profile
./toggle-display.sh --profile hp_home
./toggle-display.sh --profile hp_home --apply layout1
```

Setup is explicit once, but does not require manually copying screen ids or
editing shell configuration.
