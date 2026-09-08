# Go Migration and Profile Implementation Plan

## Objective

Replace the current Bash/A-B/HP-DELL model with a small Go program that keeps the
normal workflow simple:

```sh
toggle-display
```

The program automatically detects the connected physical monitor, finds its
monitor profile, identifies the current layout, and applies the next layout.
Profiles remain an internal concept during daily use.

The detailed data model is described in [`new-design.md`](new-design.md).

## User-facing model

A **monitor profile** is a friendly name attached to one physical monitor's
screen identifiers:

```text
hp_home
hp_office
dell_office
```

A profile contains one or more **layouts**. A layout has:

- a short stable id used by commands, such as `layout1`;
- a short friendly name, such as `under-monitor`;
- an optional longer description;
- the captured display arrangement.

The monitor id selects the profile automatically. A `place=home` tag may be
stored for humans, but it is not used as an identity because software cannot
reliably determine physical location.

## Command design

### Normal use

```sh
toggle-display
```

This should:

1. Query `displayplacer list` once.
2. Identify the connected external monitor.
3. Match it to exactly one stored profile.
4. Detect the currently active layout from the display arrangement.
5. Apply the next layout in that profile's explicit order.
6. Print the profile and layout names.

Example output:

```text
hp_home -> layout2 (under-monitor): External monitor above the laptop
```

If no profile matches, or more than one profile matches, the program must refuse
to guess and print an actionable error.

### Setup and maintenance

The profile name is needed when creating a profile, but not for normal toggling.
The first save creates and attaches a profile to the currently connected
monitor:

```sh
toggle-display --profile hp_home \
  --save layout1 \
  --name next-to-laptop \
  --description "External monitor on the left, laptop on the right"
```

After rearranging the displays, save another layout:

```sh
toggle-display --profile hp_home \
  --save layout2 \
  --name under-monitor \
  --description "External monitor above the laptop"
```

Saving an existing layout id updates it. The current arrangement is captured;
it is not applied as part of saving.

Useful explicit commands:

```sh
# Toggle a named profile rather than auto-detecting it
toggle-display --profile hp_home

# Apply one layout by its short id
toggle-display --profile hp_home --apply layout1

# List profiles, ids, tags, and layouts
toggle-display --list
toggle-display --profile hp_home --list
```

The exact flag spelling can be finalized during implementation, but the key
rule is that `--profile` is optional for ordinary use and `--save` takes a
layout id, not a profile name.

Optional metadata flags:

```sh
toggle-display --profile hp_home --tag place=home --tag model="HP 27f 4k"
```

Tags should not be required to make a profile work.

## Configuration format

Use a versioned JSON file so Go can parse it with the standard library and
friendly names can contain hyphens, spaces, and punctuation:

```text
~/.config/toggle-display/config.json
```

Proposed shape:

```json
{
  "version": 1,
  "laptop_ids": [
    {"type": "serial", "value": "s4251086178"},
    {"type": "persistent", "value": "37D8832A-2D66-02CA-B9F7-8F30A301B230"}
  ],
  "profiles": [
    {
      "id": "hp_home",
      "tags": {"place": "home", "model": "HP 27f 4k"},
      "monitor_ids": [
        {"type": "persistent", "value": "6F9FB1D9-2284-44F6-8357-9B84666EEBD5"}
      ],
      "layout_order": ["layout1", "layout2"],
      "layouts": [
        {
          "id": "layout1",
          "name": "next-to-laptop",
          "description": "External monitor on the left, laptop on the right",
          "laptop_origin": "(2560,458)",
          "displayplacer_args": [
            "id:EXTERNAL_ID res:2560x1440 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0",
            "id:LAPTOP_ID res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(2560,458) degree:0"
          ]
        }
      ]
    }
  ]
}
```

The actual Go structs may use typed identifier and layout structures rather than
this exact JSON shape. Layout order must be explicit and must not depend on
filesystem ordering.

Do not source configuration as Bash. Do not use `eval` to apply a saved layout.
Store each `displayplacer` screen specification as an argument and execute it
with `exec.Command("displayplacer", args...)`.

Writes should be atomic: write a temporary file in the same directory, fsync or
close it successfully, then rename it over the old config.

## Display discovery and matching

### Parse `displayplacer list`

Create a parser that converts the command output into typed display records:

- persistent screen id;
- serial screen id;
- display type/name;
- resolution, refresh rate, color depth, scaling;
- origin and rotation;
- enabled state;
- the current displayplacer argument/specification.

The parser must be tested with checked-in output fixtures. It should tolerate
additional mode-list lines and harmless output wording changes where possible.

### Identify the laptop and external monitor

The current MacBook ids should be migrated into `laptop_ids`. The built-in screen
can also be recognized from the display type, but an id list remains the safer
fallback.

The initial implementation should support one connected external monitor. If
zero or multiple external monitors are present, return a clear error rather than
choosing arbitrarily. Multiple-monitor support can be added later.

### Match a monitor profile

Match all registered identifiers, with this policy:

1. Prefer real unique serial ids.
2. Use persistent ids as fallback.
3. Ignore generic serial values such as `s0` when they are not unique.
4. Require exactly one profile match in automatic mode.
5. When `--profile NAME` is supplied, verify that the connected monitor belongs
   to that profile before applying it.

The first profile save records the useful identifiers currently reported for the
connected monitor. It must not record a generic serial as the only identity.

## Layout detection and application

### Detect the active layout

Do not maintain a state file. Compare the current arrangement with the layouts
stored in the selected profile.

The first version can use the laptop origin as the discriminator, provided the
profile validates that each layout has a distinct origin. Prefer a normalized
arrangement fingerprint when practical, including each screen's origin, mode,
scaling, rotation, enabled state, and screen role. Screen ids in fingerprints
must be normalized to `EXTERNAL_ID` and `LAPTOP_ID` so id churn does not make a
layout unusable.

If the current arrangement is not recognized, apply the first layout in
`layout_order` and report that the state was unknown.

### Apply a layout

Stored `displayplacer_args` contain role placeholders. Replace them with the
currently connected laptop and external ids, then run:

```go
exec.Command("displayplacer", args...).Run()
```

Return the actual exit status. Only print a successful layout message after
`displayplacer` succeeds. Capture and include useful stderr on failure.

## Migration strategy

1. **Freeze current behavior.** Keep the existing Bash script working while the
   Go implementation is developed. Do not delete the current config or layouts.
2. **Add Go scaffolding.** Create `go.mod`, a command package, and packages for
   config, displayplacer parsing, matching, and layout application. Use only Go
   standard-library dependencies initially.
3. **Implement parser fixtures.** Capture representative `displayplacer list`
   output for the current HP monitor, the Dell monitor, a disconnected monitor,
   and an unknown/new monitor.
4. **Implement JSON config and atomic writes.** Add validation for profile ids,
   layout ids, duplicate ids, duplicate monitor matches, and invalid layout
   order.
5. **Implement profile-aware save/apply/toggle.** Make the normal no-argument
   command automatically select a profile. Add the setup commands described
   above.
6. **Import existing configuration.** Provide either a one-time migration
   command or a small import helper that copies the current built-in layouts,
   known monitor ids, and valid saved layouts into `config.json`. Preserve the
   original Bash config as a backup.
7. **Install the Go command.** Use the Makefile or a documented `go build`
   command to install the binary under `~/.local/bin`.
8. **Switch the default entry point.** After manual verification on the HP home,
   HP office, and Dell setups, make the Go command the default entry point.
9. **Remove legacy behavior later.** Once the JSON profiles are confirmed,
   remove the hardcoded HP/DELL arrays, A-D terminology, sourced Bash config,
   and `eval` path. Keep a documented backup/import path for old users.

## Suggested Go project structure

```text
cmd/toggle-display/main.go
internal/config/config.go
internal/config/json.go
internal/displayplacer/list.go
internal/displayplacer/parser.go
internal/displayplacer/apply.go
internal/profile/match.go
internal/profile/toggle.go
go.mod
```

Keep the domain logic independent from the process runner. Tests should be able
to inject a fake `displayplacer list` result and a fake apply command without
requiring physical monitors.

## Test plan

### Unit tests

- Parse persistent, serial, origin, mode, scaling, rotation, and
  enabled fields.
- Parse the final displayplacer arrangement command.
- Normalize screen ids into laptop/external roles.
- Ignore generic serial ids such as `s0` when appropriate.
- Match one profile, no profiles, and ambiguous profiles.
- Detect each layout and unknown arrangements.
- Cycle layout order, including one-layout and three-layout profiles.
- Substitute ids without invoking a shell.
- Validate profile/layout ids and descriptions.
- Read and atomically write JSON configuration.

### Integration tests

Use a fake `displayplacer` executable or an injected runner to verify:

- `toggle-display` auto-selects the correct monitor profile.
- `--profile hp_home --save layout1` creates the profile and captures the
  current arrangement.
- Saving layout2 and toggling from layout1 applies layout2.
- Applying a layout returns failure when displayplacer fails.
- An office monitor does not use the home profile.
- Existing global A-D layouts remain available during migration.

### Manual acceptance tests

1. Save two layouts for the home HP monitor.
2. Run `toggle-display` twice and verify the layout names alternate.
3. Disconnect the home monitor and connect the office monitor.
4. Verify automatic matching selects the office profile.
5. Reconnect the home monitor and verify its layouts are selected again.
6. Verify a monitor with an unknown id is never changed automatically.
7. Verify `--list` explains all profiles and layouts clearly.

## Acceptance criteria

The migration is complete when:

- Daily use requires only `toggle-display`.
- Different physical HP monitors can have independent layouts without editing
  the source code.
- Layouts have short ids and descriptive output.
- Profile and layout names support hyphens and spaces where appropriate.
- No saved configuration is executed through Bash `source` or `eval`.
- A failed `displayplacer` call produces a non-zero exit status.
- Parser, matcher, toggle, and config behavior are covered by automated tests.
- The `toggle-display` command remains usable.
