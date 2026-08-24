#!/bin/bash

# --- Known external monitor persistent IDs (per monitor type) ---
# Add new IDs here when swapping monitors (run `displayplacer list` to find yours)
KNOWN_HP_IDS=(
  "06821F68-21CC-4370-8CC0-BE95ACB3AC1C"  # Old HP monitor
  "3C4D0074-0F3D-47DD-AECB-1B80731B9B3F"  # Current HP monitor
)
KNOWN_DELL_IDS=(
  "0E27842A-238C-4F65-80FB-4919641DB78B"  # Old Dell monitor
  "4BBE0CEB-FD34-4B58-AA4B-B701217F35EA"  # Current Dell monitor (S2725DC)
)

# Parse command line arguments
DISPLAY_TYPE="${1:-HP}"  # Default to HP if no argument provided

LAYOUT_FILE="/tmp/current_display_layout_${DISPLAY_TYPE}"

# Validate display type
if [[ "$DISPLAY_TYPE" != "HP" && "$DISPLAY_TYPE" != "DELL" ]]; then
    echo "Error: Invalid display type '$DISPLAY_TYPE'. Valid options are: HP, DELL"
    echo "Usage: $0 [HP|DELL]"
    exit 1
fi

# --- Find the currently connected external monitor ID for the requested type ---
find_external_id() {
    local display_type="$1"
    local known_ids=()
    local connected_ids
    local known_id

    case "$display_type" in
        HP)   known_ids=("${KNOWN_HP_IDS[@]}") ;;
        DELL) known_ids=("${KNOWN_DELL_IDS[@]}") ;;
    esac

    connected_ids=$(displayplacer list 2>/dev/null | grep "^Persistent screen id:" | awk '{print $4}')
    for known_id in "${known_ids[@]}"; do
        if echo "$connected_ids" | grep -q "$known_id"; then
            echo "$known_id"
            return 0
        fi
    done
    echo ""
    return 1
}

EXTERNAL_ID=$(find_external_id "$DISPLAY_TYPE")
if [[ -z "$EXTERNAL_ID" ]]; then
    echo "Error: No known $DISPLAY_TYPE monitor found."
    echo "Connected display IDs:"
    displayplacer list 2>/dev/null | grep "^Persistent screen id:" || echo "  (none detected)"
    echo ""
    echo "Add the desired ID to KNOWN_${DISPLAY_TYPE}_IDS at the top of this script."
    exit 1
fi

# --- Display Layout Configurations (templates with EXTERNAL_ID placeholder) ---
LAYOUT_A_TEMPLATE='displayplacer "id:EXTERNAL_ID res:2560x1440 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0" "id:37D8832A-2D66-02CA-B9F7-8F30A301B230 res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(2560,458) degree:0"'
LAYOUT_B_TEMPLATE='displayplacer "id:EXTERNAL_ID res:2560x1440 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0" "id:37D8832A-2D66-02CA-B9F7-8F30A301B230 res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(542,1440) degree:0"'
LAYOUT_C_TEMPLATE='displayplacer "id:37D8832A-2D66-02CA-B9F7-8F30A301B230 res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0" "id:EXTERNAL_ID res:2560x1440 hz:144 color_depth:8 enabled:true scaling:off origin:(1512,-458) degree:0"'
LAYOUT_D_TEMPLATE='displayplacer "id:EXTERNAL_ID res:2560x1440 hz:144 color_depth:8 enabled:true scaling:off origin:(0,0) degree:0" "id:37D8832A-2D66-02CA-B9F7-8F30A301B230 res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(524,1440) degree:0"'

if [[ "$DISPLAY_TYPE" == "HP" ]]; then
    # HP display: Toggle between Layout A and B
    if [ ! -f "$LAYOUT_FILE" ] || [ "$(cat "$LAYOUT_FILE")" = "B" ]; then
        eval "${LAYOUT_A_TEMPLATE//EXTERNAL_ID/$EXTERNAL_ID}"
        echo "A" > "$LAYOUT_FILE"
        echo "Switched to HP Layout A: Side by side"
    else
        eval "${LAYOUT_B_TEMPLATE//EXTERNAL_ID/$EXTERNAL_ID}"
        echo "B" > "$LAYOUT_FILE"
        echo "Switched to HP Layout B: Laptop under monitor"
    fi
elif [[ "$DISPLAY_TYPE" == "DELL" ]]; then
    # DELL display: Toggle between Layout C and D
    if [ ! -f "$LAYOUT_FILE" ] || [ "$(cat "$LAYOUT_FILE")" = "D" ]; then
        eval "${LAYOUT_C_TEMPLATE//EXTERNAL_ID/$EXTERNAL_ID}"
        echo "C" > "$LAYOUT_FILE"
        echo "Switched to DELL Layout C: Side by side, laptop on the left with 144Hz"
    else
        eval "${LAYOUT_D_TEMPLATE//EXTERNAL_ID/$EXTERNAL_ID}"
        echo "D" > "$LAYOUT_FILE"
        echo "Switched to DELL Layout D: Laptop under monitor with 144Hz"
    fi
fi
