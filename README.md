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

You need to record the `displayplacer` command for each layout.

**For a side-by-side layout (A / C):**
1. Go to **System Settings → Displays** and drag your screens into the side-by-side arrangement
2. Run the following in your terminal:
```sh
displayplacer list
```
3. Copy the last line of the output — it starts with `displayplacer "id:..."`

**For a vertical layout (B / D — laptop under monitor):**
1. Go back to **System Settings → Displays** and drag the laptop screen below the monitor
2. Run `displayplacer list` again and copy the last line

### 3. Update the toggle script

Edit `~/projects/toggle-display/toggle-display.sh`:
```sh
nvim ~/projects/toggle-display/toggle-display.sh
```

Set the layout templates to the commands you copied above, replacing the **external monitor's** id with the literal placeholder `EXTERNAL_ID` (the script substitutes the connected monitor's real id at runtime):
```sh
LAYOUT_A_TEMPLATE='displayplacer "id:EXTERNAL_ID res:2560x1440 hz:60 color_depth:8 scaling:on origin:(0,0) degree:0" "id:YYYY res:1800x1169 scaling:on origin:(2560,0) degree:0"'
LAYOUT_B_TEMPLATE='displayplacer "id:EXTERNAL_ID res:2560x1440 hz:60 color_depth:8 scaling:on origin:(0,0) degree:0" "id:YYYY res:1800x1169 scaling:on origin:(380,1440) degree:0"'
```

`A`/`B` are the HP layouts, `C`/`D` the Dell ones. `YYYY` is your laptop's own screen id, which stays hardcoded.

### 4. Register your monitor IDs

macOS assigns each monitor a persistent id, and it changes when you swap monitors or move to a different port. The script matches whichever known id is currently connected, so you only need to add the new id — the layouts themselves need no edits.

Run `displayplacer list`, copy the `Persistent screen id:` of your external monitor, and add it to the matching array at the top of the script:
```sh
KNOWN_HP_IDS=(
  "3C4D0074-0F3D-47DD-AECB-1B80731B9B3F"  # Current HP monitor
)
KNOWN_DELL_IDS=(
  "4BBE0CEB-FD34-4B58-AA4B-B701217F35EA"  # Current Dell monitor (S2725DC)
)
```

If no id in the relevant array is connected, the script prints the connected ids and exits without touching your screens.

## Usage

### From the terminal

```sh
# HP monitor (default) — toggles A <-> B
~/projects/toggle-display/toggle-display.sh

# Dell monitor — toggles C <-> D
~/projects/toggle-display/toggle-display.sh DELL
```

Or, to run it from anywhere, add an alias to your shell config (`~/.zshrc` or `~/.bashrc`):

```sh
# zhrc
echo 'alias toggle-display="~/projects/toggle-display/toggle-display.sh"' >> ~/.zshrc

echo 'alias toggle-display="~/projects/toggle-display/toggle-display.sh"' >> ~/.oh-my-zsh/custom/{CUSTOM_FILENAME}.zsh
source ~/.zshrc
```

Then just type:
```sh
toggle-display        # HP
toggle-display DELL   # Dell
```

---

## Troubleshooting

**`displayplacer: command not found`**
Run `brew install displayplacer` and make sure Homebrew's bin is in your PATH.

**`Error: No known HP/DELL monitor found`**
The monitor's persistent id isn't registered. Copy the `Persistent screen id:` from the printed list and add it to `KNOWN_HP_IDS` or `KNOWN_DELL_IDS` at the top of the script.

**Layout doesn't apply correctly after waking from sleep**
This is a known macOS quirk where screen IDs can change. Re-run `displayplacer list` with both screens connected and add the new id to the relevant `KNOWN_*_IDS` array.

**The script toggles but nothing changes visually**
Make sure both screens are connected and awake before running the script. Check that the screen IDs in your commands match the output of `displayplacer list`.

---

## Project Structure

```
~/projects/toggle-display/
├── README.md
└── toggle-display.sh
```

---

## Credits

- [displayplacer](https://github.com/jakehilborn/displayplacer) by Jake Hilborn — MIT License
