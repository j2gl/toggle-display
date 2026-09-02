# Shell Completion Plan

## Objective

Provide fast, context-aware completion for `toggle-display` without querying or
modifying physical displays.

The first implementation should target zsh because it is the default shell on
current macOS installations. The design should leave room for Bash and Fish
completion later.

## Proposed interface

Add a completion-output flag:

```sh
toggle-display --completion zsh
toggle-display --completion bash
toggle-display --completion fish
```

The command prints a completion script to stdout and performs no display
queries. Installation remains the user's choice, for example:

```sh
mkdir -p ~/.zfunc
toggle-display --completion zsh > ~/.zfunc/_toggle-display
# Add ~/.zfunc to fpath before compinit in ~/.zshrc.
autoload -Uz compinit && compinit
```

The generated function should call a machine-readable, read-only completion
endpoint when it needs current profile or layout ids. Do not parse the human
output of `--list`.

Proposed internal endpoint:

```sh
toggle-display --complete profiles
toggle-display --complete layouts --profile hp_home
```

Each command prints one id per line, in stable order, and does not invoke
`displayplacer`. Errors go to stderr and produce no completion candidates.
`--complete` is intended for completion scripts and can remain documented as an
advanced interface.

## Completion behavior

### Options

Complete the supported options:

- `--profile`
- `--save`
- `--apply`
- `--name`
- `--description`
- `--tag`
- `--list`
- `--migrate`
- `--force`
- `--config`
- `--legacy-config`
- `--completion`
- `--help`

Do not suggest `--name` or `--description` values; they are free-form text.

### Profiles

After either form of the profile option, complete configured profile ids:

```sh
toggle-display --profile <TAB>
toggle-display --profile=<TAB>
```

Profile candidates come from the configured JSON file and must work with
`XDG_CONFIG_HOME`, the normal config path, and an explicit `--config PATH` when
that path appears earlier in the command line.

### Layouts

After `--apply` or `--save`, complete layout ids for the selected profile:

```sh
toggle-display --profile hp_home --apply <TAB>
toggle-display --profile hp_home --save <TAB>
```

If no profile has been supplied, do not invoke display discovery. Either return
no layout candidates or offer ids from all profiles only if that behavior is
clearly labeled; the preferred behavior is to prompt the user to complete
`--profile` first.

### Tags and paths

Do not try to infer tag keys from display hardware. Standard path completion
should remain available for `--config` and `--legacy-config` values.

## Implementation steps

1. Add a `--completion` flag and reject unsupported shell names with a useful
   error.
2. Add `--complete profiles` and `--complete layouts --profile NAME`.
3. Make the completion endpoints load and validate JSON only; they must never
   construct a displayplacer runner.
4. Generate the zsh function using zsh's `_arguments` and `_describe`
   conventions.
5. Handle `--profile VALUE` and `--profile=VALUE`, including the option order
   used when `--config PATH` precedes the profile.
6. Add a short installation section to `README.md` and keep generated output
   deterministic.
7. Add Bash and Fish generators only after the zsh behavior is stable.

## Testing plan

### Go tests

- completion output is generated for zsh;
- unsupported shells fail without querying displayplacer;
- profile candidates are read from a temporary JSON config;
- layout candidates are scoped to the requested profile;
- ids are sorted deterministically;
- malformed or missing config produces no candidates and a useful error;
- completion commands do not call the injected displayplacer runner.

### Shell tests

- install the generated zsh function in a temporary zsh environment;
- verify option, profile, and layout completion syntax;
- verify both `--profile VALUE` and `--profile=VALUE` forms;
- run ShellCheck on the generated script and the compatibility wrapper where
  available.

### Manual acceptance

1. Install the completion function in zsh.
2. Type `toggle-display --profile ` and confirm profile ids appear.
3. Type `toggle-display --profile hp_home --apply ` and confirm only that
   profile's layout ids appear.
4. Confirm completion works while no monitor is connected.
5. Confirm completion never runs `displayplacer list` or applies a layout.
6. Edit the JSON config, start a new completion request, and confirm candidates
   reflect the change.

## Safety and compatibility

- Completion must be read-only and side-effect free.
- It must not source JSON or any other configuration as shell code.
- It must not use `eval` to construct candidates or commands.
- Friendly names and descriptions should not be treated as ids.
- A missing or invalid config must fail closed rather than suggesting stale
  hardware operations.
- The existing no-argument toggle workflow must remain unchanged.
