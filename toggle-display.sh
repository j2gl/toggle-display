#!/bin/bash
#
# toggle-display — switch between saved display arrangements from the CLI.
# Design rationale and open questions: doc/hardening-plan.md

# --- Known display identifiers -------------------------------------------------
# Any id in a list identifies that monitor. Serial ids (sNNNNN) are tied to the
# display hardware and survive port changes and wake races, so they are listed
# first; persistent ids stay as fallbacks. Run `displayplacer list` to find yours.
LAPTOP_IDS=(
  "s4251086178"                           # MacBook built-in screen
  "37D8832A-2D66-02CA-B9F7-8F30A301B230"
)
HP_IDS=(
  "s16843009"                             # HP 28" — placeholder EDID serial (0x01010101)
  "06821F68-21CC-4370-8CC0-BE95ACB3AC1C"
  "3C4D0074-0F3D-47DD-AECB-1B80731B9B3F"
)
DELL_IDS=(
  "s1093808706"                           # Dell S2725DC
  "4BBE0CEB-FD34-4B58-AA4B-B701217F35EA"
  "0E27842A-238C-4F65-80FB-4919641DB78B"  # older Dell
)

# --- Layout definitions --------------------------------------------------------
# EXTERNAL_ID and LAPTOP_ID are placeholders, substituted with the ids actually
# connected. *_ORIGIN is the laptop's origin in that layout — the fingerprint used
# to detect which layout is currently applied, so all four must stay distinct.
LAYOUT_A_TEMPLATE='displayplacer "id:EXTERNAL_ID res:2560x1440 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0" "id:LAPTOP_ID res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(2560,458) degree:0"'
LAYOUT_A_ORIGIN='(2560,458)'
LAYOUT_A_DESC='HP Layout A: Side by side'

LAYOUT_B_TEMPLATE='displayplacer "id:EXTERNAL_ID res:2560x1440 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0" "id:LAPTOP_ID res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(542,1440) degree:0"'
LAYOUT_B_ORIGIN='(542,1440)'
LAYOUT_B_DESC='HP Layout B: Laptop under monitor'

LAYOUT_C_TEMPLATE='displayplacer "id:LAPTOP_ID res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0" "id:EXTERNAL_ID res:2560x1440 hz:144 color_depth:8 enabled:true scaling:off origin:(1512,-458) degree:0"'
LAYOUT_C_ORIGIN='(0,0)'
LAYOUT_C_DESC='DELL Layout C: Side by side, laptop on the left with 144Hz'

LAYOUT_D_TEMPLATE='displayplacer "id:EXTERNAL_ID res:2560x1440 hz:144 color_depth:8 enabled:true scaling:off origin:(0,0) degree:0" "id:LAPTOP_ID res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(524,1440) degree:0"'
LAYOUT_D_ORIGIN='(524,1440)'
LAYOUT_D_DESC='DELL Layout D: Laptop under monitor with 144Hz'

# Layouts toggled for each monitor, as "primary secondary"
HP_LAYOUTS="A B"
DELL_LAYOUTS="C D"

# --- User config ---------------------------------------------------------------
# Sourced after the defaults above, so it can override any layout or id list, and
# holds anything written by --save.
CONFIG_FILE="${XDG_CONFIG_HOME:-$HOME/.config}/toggle-display/layouts.conf"
[ -f "$CONFIG_FILE" ] && . "$CONFIG_FILE"

usage() {
    cat <<EOF
Usage: $(basename "$0") [HP|DELL] [--save NAME] [--apply NAME] [--help]

  (no arguments)   Detect the connected monitor and toggle its two layouts
  HP               Toggle the HP layouts   ($HP_LAYOUTS)
  DELL             Toggle the Dell layouts ($DELL_LAYOUTS)
  --apply NAME     Apply a named layout directly
  --save NAME      Save the current arrangement as NAME into
                   $CONFIG_FILE
                   Saving as an existing name (A-D) overrides that layout.
  --help           Show this message

The active layout is detected by reading the current arrangement, so the toggle
stays correct after a reboot or after rearranging screens in System Settings.
EOF
}

# --- Display queries -----------------------------------------------------------
# `displayplacer list` is captured once so every query sees the same snapshot.
if ! command -v displayplacer >/dev/null 2>&1; then
    echo "Error: displayplacer not found. Install it with: brew install displayplacer" >&2
    exit 1
fi
DISPLAY_LIST=$(displayplacer list 2>/dev/null)

# Echo the first of the given ids that is currently connected.
find_connected() {
    local id
    for id in "$@"; do
        if printf '%s\n' "$DISPLAY_LIST" | grep -q "screen id: ${id}\$"; then
            echo "$id"
            return 0
        fi
    done
    return 1
}

# Echo a connected screen's origin, e.g. "(2560,458)". Empty if not connected,
# which doubles as the presence check.
current_origin() {
    printf '%s\n' "$DISPLAY_LIST" | awk -v id="$1" '
        $0 ~ "screen id: " id "$" { found=1 }
        found && /^Origin:/ { print $2; exit }
    '
}

# Copy a monitor type's id list into TYPE_IDS (bash 3.2 has no namerefs).
set_type_ids() {
    case "$1" in
        HP)   TYPE_IDS=("${HP_IDS[@]}") ;;
        DELL) TYPE_IDS=("${DELL_IDS[@]}") ;;
        *)    TYPE_IDS=() ;;
    esac
}

# Echo the monitor type that is currently connected.
detect_display_type() {
    local type
    for type in HP DELL; do
        set_type_ids "$type"
        if find_connected "${TYPE_IDS[@]}" >/dev/null; then
            echo "$type"
            return 0
        fi
    done
    return 1
}

# --- Layout actions ------------------------------------------------------------
apply_layout() {
    local name="$1"
    local template_var="LAYOUT_${name}_TEMPLATE"
    local desc_var="LAYOUT_${name}_DESC"
    local template="${!template_var}"

    if [ -z "$template" ]; then
        echo "Error: unknown layout '$name'." >&2
        exit 1
    fi

    template="${template//EXTERNAL_ID/$EXTERNAL_ID}"
    template="${template//LAPTOP_ID/$LAPTOP_ID}"
    eval "$template"
    echo "Switched to ${!desc_var:-layout $name}"
}

# Apply the secondary layout if the primary one is currently active, else the
# primary. Any unrecognized arrangement therefore falls back to the primary.
toggle_layouts() {
    local primary="$1" secondary="$2"
    local origin_var="LAYOUT_${primary}_ORIGIN"

    if [ "$(current_origin "$LAPTOP_ID")" = "${!origin_var}" ]; then
        apply_layout "$secondary"
    else
        apply_layout "$primary"
    fi
}

# Capture the current arrangement into the config file, with the connected ids
# swapped back out for placeholders so it survives future id changes.
save_layout() {
    local name="$1"
    local command origin id
    command=$(printf '%s\n' "$DISPLAY_LIST" | tail -1)
    origin=$(current_origin "$LAPTOP_ID")

    # `displayplacer list` always prints persistent ids, which may not be the id
    # form we matched on, so replace every known id for each screen.
    set_type_ids "$DISPLAY_TYPE"
    for id in "${TYPE_IDS[@]}"; do
        command="${command//$id/EXTERNAL_ID}"
    done
    for id in "${LAPTOP_IDS[@]}"; do
        command="${command//$id/LAPTOP_ID}"
    done

    mkdir -p "$(dirname "$CONFIG_FILE")"
    {
        echo ""
        echo "LAYOUT_${name}_TEMPLATE='${command}'"
        echo "LAYOUT_${name}_ORIGIN='${origin}'"
        echo "LAYOUT_${name}_DESC='layout ${name} (saved)'"
    } >> "$CONFIG_FILE"

    echo "Saved current arrangement as layout $name in:"
    echo "  $CONFIG_FILE"
}

# --- Parse arguments -----------------------------------------------------------
DISPLAY_TYPE=""
ACTION="toggle"
LAYOUT_NAME=""

while [ $# -gt 0 ]; do
    case "$1" in
        HP|DELL)
            DISPLAY_TYPE="$1"
            ;;
        --save)
            ACTION="save"
            LAYOUT_NAME="$2"
            shift
            ;;
        --apply)
            ACTION="apply"
            LAYOUT_NAME="$2"
            shift
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            echo "Error: unknown argument '$1'." >&2
            echo "" >&2
            usage >&2
            exit 1
            ;;
    esac
    shift
done

if [ "$ACTION" != "toggle" ] && [ -z "$LAYOUT_NAME" ]; then
    echo "Error: --$ACTION needs a layout name, e.g. --$ACTION A" >&2
    exit 1
fi

# --- Resolve the connected screens ---------------------------------------------
if [ -z "$DISPLAY_TYPE" ]; then
    DISPLAY_TYPE=$(detect_display_type)
    if [ -z "$DISPLAY_TYPE" ]; then
        echo "Error: no known external monitor is connected." >&2
        echo "Connected screens:" >&2
        printf '%s\n' "$DISPLAY_LIST" | grep -E "^(Persistent|Serial) screen id:" >&2 || echo "  (none detected)" >&2
        echo "" >&2
        echo "Add an id above to HP_IDS or DELL_IDS at the top of this script." >&2
        exit 1
    fi
fi

set_type_ids "$DISPLAY_TYPE"
EXTERNAL_ID=$(find_connected "${TYPE_IDS[@]}")
if [ -z "$EXTERNAL_ID" ]; then
    echo "Error: no known $DISPLAY_TYPE monitor is connected." >&2
    echo "Connected screens:" >&2
    printf '%s\n' "$DISPLAY_LIST" | grep -E "^(Persistent|Serial) screen id:" >&2 || echo "  (none detected)" >&2
    echo "" >&2
    echo "Add an id above to ${DISPLAY_TYPE}_IDS at the top of this script." >&2
    exit 1
fi

LAPTOP_ID=$(find_connected "${LAPTOP_IDS[@]}")
if [ -z "$LAPTOP_ID" ]; then
    echo "Error: the built-in screen was not found among the connected screens." >&2
    echo "Add its id to LAPTOP_IDS at the top of this script." >&2
    exit 1
fi

# --- Run -----------------------------------------------------------------------
case "$ACTION" in
    save)
        save_layout "$LAYOUT_NAME"
        ;;
    apply)
        apply_layout "$LAYOUT_NAME"
        ;;
    toggle)
        if [ "$DISPLAY_TYPE" = "HP" ]; then
            toggle_layouts $HP_LAYOUTS
        else
            toggle_layouts $DELL_LAYOUTS
        fi
        ;;
esac
