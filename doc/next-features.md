# Next Features

This document tracks the next improvements after the Go migration. The current
implementation already supports profile-aware toggling, JSON configuration,
parser fixtures, migration, and shell-wrapper compatibility.

## Priorities

### 1. Shell completion

Add context-aware completion for the installed `toggle-display` command:

- profile ids after `--profile`;
- layout ids after `--apply` and `--save`;
- supported options and their values;
- zsh first, followed by Bash and Fish if useful.

Completion must read configuration only. It must not invoke `displayplacer list`
or change displays. See [shell-completion-plan.md](shell-completion-plan.md).

### 2. `--doctor` diagnostics

Add a read-only diagnostic command that reports:

- detected displays and all observed identifiers;
- which display was identified as the laptop;
- the connected external monitor;
- matching profile candidates and match strength;
- the current laptop origin and recognized layout;
- configuration or duplicate-identity problems.

This should make setup and recovery possible without interpreting raw
`displayplacer list` output.

### 3. Monitor rebinding

Add an explicit operation for replacing a monitor or repairing changed ids, for
example:

```sh
toggle-display --profile hp_home --rebind
```

Rebinding must be deliberate, display the new identifiers, and refuse to alter
the profile unless the user confirms the operation. It should not be part of
automatic toggling.

### 4. Stronger layout fingerprints

Keep laptop origin as the fallback discriminator, but add a normalized
arrangement fingerprint when practical. Include, per screen:

- role (`LAPTOP_ID` or `EXTERNAL_ID`);
- origin;
- resolution and refresh rate;
- color depth and scaling;
- rotation and enabled state.

Screen ids must be normalized to roles so persistent-id churn does not make a
saved layout unusable. Profiles must continue to reject ambiguous layouts.

### 5. Wake and reconnect resilience

Add a short, bounded retry policy around `displayplacer list` for displays that
are still waking up. The retry behavior should:

- be limited to a small total timeout;
- report the final observed identifiers on failure;
- never apply a layout from a stale or partial snapshot.

### 6. Configuration maintenance

Consider adding:

- a config format version migration framework;
- explicit config backup and restore commands;
- a read-only config validation command;
- protection against concurrent writes.

### 7. Multiple external monitors

Support this only after the single-external-monitor behavior is well tested.
The model will need screen roles beyond one `EXTERNAL_ID`, layout validation for
all connected screens, and a clear policy for selecting a profile when several
monitors are present.

## Suggested sequence

1. Shell completion.
2. `--doctor` and read-only validation.
3. Monitor rebinding.
4. Stronger fingerprints and wake retries.
5. Configuration maintenance.
6. Multiple-monitor support.

Each feature should keep the no-argument workflow safe: no guessing, no shell
evaluation, and no success message before a display change succeeds.
