# toggle-display

A simple macOS shell script to toggle between two display arrangements from the command line — no mouse needed.

## Use Case

Each external monitor has its own pair of layouts, toggled independently.

**HP monitor** (`toggle-display HP`, the default):
- **Layout A – Side by side:** Laptop on the right, external monitor on the left
- **Layout B – Vertical:** External monitor on top, laptop display underneath (ideal for meetings/camera use)

**Dell monitor** (`toggle-display DELL`) — runs at 144Hz, unscaled:
- **Layout C – Side by side:** Laptop on the left (main display), monitor on the right
- **Layout D – Vertical:** Monitor on top (main display), laptop centred underneath

---

## Dependencies

### 1. Homebrew
The macOS package manager. Required to install `displayplacer`.

Check if you already have it:
```sh
brew --version
```

If not, install it:
```sh
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

### 2. displayplacer
A macOS command line utility to configure multi-display resolutions and arrangements.

- **Author:** [Jake Hilborn](https://github.com/jakehilborn)
- **Source:** [github.com/jakehilborn/displayplacer](https://github.com/jakehilborn/displayplacer)

Install via Homebrew:
```sh
brew install displayplacer
```

Verify installation:
```sh
displayplacer --version
```

---

## Project Setup

### 1. Clone or create the project folder

```sh
mkdir -p ~/projects/toggle-display
cd ~/projects/toggle-display
```

### 2. Capture your display layouts

Arrange your screens in **System Settings → Displays**, then save the arrangement
under a name:

```sh
# Side-by-side arrangement
./toggle-display.sh --save A

# Then drag the laptop below the monitor and save the second one
./toggle-display.sh --save B
```

`--save` writes to `~/.config/toggle-display/layouts.conf`, swapping the connected
screen ids out for placeholders so the layout keeps working after macOS reshuffles
ids. Saved layouts are associated with the currently connected physical monitor,
so different HP monitors can each have their own `A`/`B` pair. Layout names may
contain letters, numbers, and underscores, but cannot start with a number. Saving
under an existing name (`A`–`D`) overrides that layout for the current monitor;
monitors without an override use the built-in layout.

`A`/`B` are the HP layouts, `C`/`D` the Dell ones.

### 3. Register your monitor IDs

macOS assigns each screen several ids. The script accepts **any** known id per
monitor, so it keeps working when one of them changes:

```sh
HP_IDS=(
  "s16843009"                             # serial id — survives port changes
  "06821F68-21CC-4370-8CC0-BE95ACB3AC1C"  # persistent id — fallback
  "6F9FB1D9-2284-44F6-8357-9B84666EEBD5"  # persistent id currently reported by macOS
)
DELL_IDS=(
  "s1093808706"                           # Dell S2725DC
  "4BBE0CEB-FD34-4B58-AA4B-B701217F35EA"
)
```

Serial ids (`sNNNNN`) are tied to the display hardware, so they normally survive
port changes and wake-order races — list them first. Persistent ids stay as
fallbacks for monitors whose serial is a placeholder value.

Run `displayplacer list` to see all three id forms for each connected screen. If
no known id is connected, the script prints what it found and exits without
touching your screens.

## Usage

### From the terminal

```sh
# Detect the connected monitor and toggle its two layouts
~/projects/toggle-display/toggle-display.sh

# Or name the monitor explicitly
~/projects/toggle-display/toggle-display.sh HP     # toggles A <-> B
~/projects/toggle-display/toggle-display.sh DELL   # toggles C <-> D

# Apply one layout directly, without toggling
~/projects/toggle-display/toggle-display.sh --apply C

# Save the current arrangement for the connected monitor
~/projects/toggle-display/toggle-display.sh --save C

# Save two layouts for a new HP monitor; these override A/B only for that monitor
~/projects/toggle-display/toggle-display.sh HP --save A
# Rearrange the displays to the second preferred position, then:
~/projects/toggle-display/toggle-display.sh HP --save B
```

The active layout is detected by reading the current arrangement rather than by
remembering the last run, so the toggle stays correct after a reboot or after you
rearrange screens by hand. The connected monitor's persistent id selects any
monitor-specific saved layouts automatically. An arrangement it doesn't recognise
falls back to the first layout for that monitor.

Or, to run it from anywhere, add an alias to your shell config (`~/.zshrc` or `~/.bashrc`):

```sh
# zhrc
echo 'alias toggle-display="~/projects/toggle-display/toggle-display.sh"' >> ~/.zshrc

echo 'alias toggle-display="~/projects/toggle-display/toggle-display.sh"' >> ~/.oh-my-zsh/custom/{CUSTOM_FILENAME}.zsh
source ~/.zshrc
```

Then just type:
```sh
toggle-display        # detects the connected monitor
```

---

## Troubleshooting

**`displayplacer: command not found`**
Run `brew install displayplacer` and make sure Homebrew's bin is in your PATH.

**`Error: no known HP/DELL monitor is connected`**
None of the monitor's registered ids matched. Copy an id from the printed list and add it to `HP_IDS` or `DELL_IDS` at the top of the script — prefer the `Serial screen id:`.

**Layout doesn't apply correctly after waking from sleep**
This is a known macOS quirk where screen IDs can change. Adding the monitor's serial id to the relevant `*_IDS` array usually fixes it for good, since serial ids are tied to the display hardware rather than the port.

**The toggle goes to the wrong layout**
Shouldn't happen any more — the script reads the current arrangement instead of remembering the last run. If it does, two layouts probably share the same laptop origin, which is what the script uses to tell them apart. Check the `LAYOUT_*_ORIGIN` values are all distinct.

**The script toggles but nothing changes visually**
Make sure both screens are connected and awake before running the script. Check that the screen IDs in your commands match the output of `displayplacer list`.

---

## Project Structure

```
~/projects/toggle-display/
├── README.md
├── doc/
│   └── hardening-plan.md
└── toggle-display.sh
```

---

## Credits

- [displayplacer](https://github.com/jakehilborn/displayplacer) by Jake Hilborn — MIT License
