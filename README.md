# toggle-display

A simple macOS shell script to toggle between two display arrangements from the command line — no mouse needed.

## Use Case

Quickly switch between:
- **Layout A – Side by side:** Laptop on the right, external monitor on the left
- **Layout B – Vertical:** External monitor on top, laptop display underneath (ideal for meetings/camera use)

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

You need to record the `displayplacer` command for each of your two layouts.

**For Layout A (side by side):**
1. Go to **System Settings → Displays** and drag your screens into the side-by-side arrangement
2. Run the following in your terminal:
```sh
displayplacer list
```
3. Copy the last line of the output — it starts with `displayplacer "id:..."`

**For Layout B (vertical — laptop under monitor):**
1. Go back to **System Settings → Displays** and drag the laptop screen below the monitor
2. Run `displayplacer list` again and copy the last line

### 3. Update the toggle script

Edit `~/projects/toggle-display/toggle-display.sh` and set the `LAYOUT_A` and `LAYOUT_B` variables.
```sh
LAYOUT_A='displayplacer "id:XXXX res:2560x1440 hz:60 color_depth:8 scaling:on origin:(0,0) degree:0" "id:YYYY res:1800x1169 scaling:on origin:(2560,0) degree:0"'
LAYOUT_B='displayplacer "id:XXXX res:2560x1440 hz:60 color_depth:8 scaling:on origin:(0,0) degree:0" "id:YYYY res:1800x1169 scaling:on origin:(380,1440) degree:0"'
```

Edit the file:
```sh
nvim ~/projects/toggle-display/toggle-display.sh
```

Open it in your editor and paste the following, replacing the `LAYOUT_A` and `LAYOUT_B` values with the commands you copied in the previous step:

## Usage

### From the terminal

```sh
~/projects/toggle-display/toggle-display.sh
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
toggle-display
```

---

## Troubleshooting

**`displayplacer: command not found`**
Run `brew install displayplacer` and make sure Homebrew's bin is in your PATH.

**Layout doesn't apply correctly after waking from sleep**
This is a known macOS quirk where screen IDs can change. Re-run `displayplacer list` with both screens connected and update the IDs in the script.

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
