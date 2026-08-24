# Hardening Plan

Plan for making `toggle-display.sh` more robust and easier to live with, without
adding dependencies or changing language.

**Status:** proposed, not implemented. Written 2026-08-24, after adding the Dell
C/D toggle.

---

## Why

The script works, but three things need hand-maintenance or go stale:

1. **`KNOWN_*_IDS` grows forever.** macOS reshuffles persistent screen ids on
   port changes and wake races, so every reshuffle means editing the script. This
   already bit us: `LAYOUT_C` shipped with a stale Dell id
   (`0E27842A-…`) that no longer matched anything connected.
2. **The `/tmp` state file is a guess.** `/tmp/current_display_layout_<TYPE>`
   records what the script *last did*, not what the screens are *actually* doing.
   It's wiped on reboot, and it drifts the moment you rearrange displays by hand
   in System Settings. Next toggle then jumps to the wrong layout.
3. **Layouts live in code.** Adding a monitor means editing a script rather than
   editing configuration.

---

## Hardware inventory

Captured from `displayplacer list` on 2026-08-24. The serial ids are the new
information — they're tied to display hardware rather than to the port.

| Display | Persistent id | Serial id | Notes |
|---|---|---|---|
| MacBook built-in | `37D8832A-2D66-02CA-B9F7-8F30A301B230` | `s4251086178` | 1512x982 @120Hz |
| Dell S2725DC | `4BBE0CEB-FD34-4B58-AA4B-B701217F35EA` | `s1093808706` | 2560x1440 @144Hz, `scaling:off` |
| HP 28" | `06821F68-21CC-4370-8CC0-BE95ACB3AC1C` | `s16843009` | 2560x1440 @60Hz, `scaling:on` |
| HP (other) | `3C4D0074-0F3D-47DD-AECB-1B80731B9B3F` | *unknown* | not connected when captured |
| Dell (old) | `0E27842A-238C-4F65-80FB-4919641DB78B` | *unknown* | stale, kept as fallback |

Two caveats worth recording:

- **The HP's serial is a placeholder.** `s16843009` is `0x01010101` — a filler
  value, not a real EDID serial. It should still be *stable* (it's baked into the
  monitor firmware), but it is not guaranteed *unique*: any other monitor with an
  unset serial reports the same thing. `displayplacer --help` hedges on exactly
  this: "*If* the serial screenIds are unique for all of your monitors, use these."
- **The script's id comments have drifted.** The HP connected on 2026-08-24 is
  `06821F68-…`, which the script labels "Old HP monitor". Hand-maintained
  comments stop being trustworthy; don't rely on them to identify hardware.

---

## Change 1 — Detect the current layout instead of remembering it

**Highest value, no caveats, and it deletes code.**

Each of the four layouts puts the *laptop* in a distinct position, so the
laptop's origin alone identifies which layout is live:

| Layout | Monitor | Arrangement | Laptop origin |
|---|---|---|---|
| A | HP | side by side | `(2560,458)` |
| B | HP | laptop under monitor | `(542,1440)` |
| C | Dell | side by side | `(0,0)` |
| D | Dell | laptop under monitor | `(524,1440)` |

Read it straight from `displayplacer list`:

```bash
current_origin() {
    displayplacer list 2>/dev/null | awk -v id="$1" '
        $0 ~ "screen id: " id "$" { found=1 }
        found && /^Origin:/ { print $2; exit }
    '
}
```

Verified working against the live setup on 2026-08-24: returned `(2560,458)` for
the laptop, correctly identifying Layout A as applied. The `- main display`
suffix on the origin line is harmless — taking `$2` drops it.

The toggle becomes:

```bash
if [ "$(current_origin "$LAPTOP_ID")" = "$LAYOUT_A_LAPTOP_ORIGIN" ]; then
    apply B
else
    apply A     # any unrecognized state falls back to the primary layout
fi
```

What this buys:

- **`LAYOUT_FILE` and all `/tmp` handling disappear.** No reboot staleness, no
  drift after manual rearranging.
- **Self-correcting.** An unrecognized arrangement falls back to the primary
  layout rather than toggling to the wrong one.
- **It doubles as the presence check.** `current_origin` returns empty for a
  disconnected monitor (confirmed against the unplugged Dell), so this one
  function replaces the whole `find_external_id` loop.

Constraint to preserve: layouts must stay distinguishable by laptop origin. The
current four are. The fallback covers anything else.

## Change 2 — Match on any known id, not one id type

Given the HP's placeholder serial, don't commit to a single id type. Give each
monitor a list of identifiers and check all of them against every `screen id:`
line in the output:

```bash
# Any of these identify the monitor. Serial ids first — they survive port changes.
LAPTOP_IDS=("s4251086178" "37D8832A-2D66-02CA-B9F7-8F30A301B230")
HP_IDS=("s16843009" "06821F68-21CC-4370-8CC0-BE95ACB3AC1C" "3C4D0074-0F3D-47DD-AECB-1B80731B9B3F")
DELL_IDS=("s1093808706" "4BBE0CEB-FD34-4B58-AA4B-B701217F35EA")
```

Structurally this is the array that already exists — what changes is what it
means. Today the list grows every time macOS churns a persistent id. With a
serial in it, the serial matches first and the list only changes when a monitor
is *bought*. Append-often becomes append-almost-never, and the placeholder-serial
risk stays contained because the persistent ids remain as fallbacks.

The `EXTERNAL_ID` placeholder substitution in the layout templates stays exactly
as it is — it just gets fed a better-resolved id.

## Change 3 — Config file plus `--save`

**Lowest value of the three, and only worth doing as a pair.** A config file on
its own just moves hand-editing from one file to another; it pays off only once
something *writes* it.

Making the config a sourced bash file removes the parsing problem entirely, which
is what makes `--save` cheap:

```bash
CONFIG="${XDG_CONFIG_HOME:-$HOME/.config}/toggle-display/layouts.conf"
[ -f "$CONFIG" ] && . "$CONFIG"        # reading: free

save_layout() {                         # writing: ~4 lines
    mkdir -p "$(dirname "$CONFIG")"
    echo "LAYOUT_$1='$(displayplacer list 2>/dev/null | tail -1)'" >> "$CONFIG"
}
```

The final line of `displayplacer list` is already a valid command, so capturing
the current arrangement is just `tail -1`.

Sourcing the config means arbitrary code execution from that file. It's a
user-owned file in `$HOME`, so the practical risk is nil — but it is the reason
this is a sourced `.conf` and not a parsed format.

Fiddly part: each saved layout also needs its discriminator origin recorded, or
Change 1 can't identify it.

---

## Order and effort

1. **Change 1** — biggest behavioral win, needs no hardware to test, removes code.
2. **Change 2** — largely a rename plus adding the two known serial ids.
3. **Change 3** — only if named profiles beyond A–D are wanted.

Changes 1 and 2 together are roughly 30 minutes and leave the script shorter than
it is now. Change 3 is about an hour and is where scope starts to grow.

---

## Out of scope — what this does not fix

- **Genuinely colliding serials.** Two monitors both reporting an unset serial
  would be indistinguishable by serial id (persistent-id fallback covers it).
- **macOS rejecting a mode.** If the requested resolution/refresh isn't
  available, `displayplacer` falls back to another working mode.
- **The wake-from-sleep race.** Running the toggle before a monitor finishes
  enumerating means it isn't in the list yet. Wants a retry loop — a separate
  change from these three.

---

## Considered and rejected: porting to Go

Rejected for now. A Go version would still shell out to `displayplacer`, so every
failure mode above stays identical — the churn, the stale state, the hardcoded
layouts are configuration and design problems, not language problems. The trade
would be a build step and a binary to distribute in exchange for nicer internals.

Go earns its keep only if:

- **The tool grows** past a handful of layouts — real flag parsing, subcommands,
  and struct-typed layouts start paying off.
- **Testability matters.** Layout-matching logic could be unit tested without the
  monitors physically attached. The bash version can't be: the HP path went
  unverified for a full session simply because the monitor was unplugged.
- **Dropping `displayplacer` entirely** becomes desirable — cgo against
  CoreGraphics (`CGConfigureDisplayWithDisplayMode` and friends) removes the
  external dependency and exposes display product names natively. This is the
  only variant that is genuinely *more robust* rather than differently written,
  and it means reimplementing work that `displayplacer` already does well.

Note that the `--save` argument for Go is weaker than it first appears: with a
sourced config, writing profiles in bash is about four lines (see Change 3).

---

## Open questions

- Is `3C4D0074-…` still in use? Its serial id needs capturing while connected —
  and confirming it doesn't collide with the HP's `s16843009`.
- Are the "Old HP monitor" / "Current HP monitor" comments still accurate? The
  connected HP on 2026-08-24 was the one labelled "Old".
- Should the old Dell (`0E27842A-…`) stay as a fallback, or be dropped?
