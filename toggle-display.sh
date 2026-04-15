#!/bin/bash

# Parse command line arguments
DISPLAY_TYPE="${1:-HP}"  # Default to HP if no argument provided

LAYOUT_FILE="/tmp/current_display_layout_${DISPLAY_TYPE}"

# Validate display type
if [[ "$DISPLAY_TYPE" != "HP" && "$DISPLAY_TYPE" != "DELL" ]]; then
    echo "Error: Invalid display type '$DISPLAY_TYPE'. Valid options are: HP, DELL"
    echo "Usage: $0 [HP|DELL]"
    exit 1
fi

# --- Display Layout Configurations ---
LAYOUT_A='displayplacer "id:06821F68-21CC-4370-8CC0-BE95ACB3AC1C res:2560x1440 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0" "id:37D8832A-2D66-02CA-B9F7-8F30A301B230 res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(2560,458) degree:0"'
LAYOUT_B='displayplacer "id:06821F68-21CC-4370-8CC0-BE95ACB3AC1C res:2560x1440 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0" "id:37D8832A-2D66-02CA-B9F7-8F30A301B230 res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(542,1440) degree:0"'
LAYOUT_C='displayplacer "id:0E27842A-238C-4F65-80FB-4919641DB78B res:2560x1440 hz:144 color_depth:8 enabled:true scaling:off origin:(0,0) degree:0" "id:37D8832A-2D66-02CA-B9F7-8F30A301B230 res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(2560,517) degree:0"'

if [[ "$DISPLAY_TYPE" == "HP" ]]; then
    # HP display: Toggle between Layout A and B
    if [ ! -f "$LAYOUT_FILE" ] || [ "$(cat $LAYOUT_FILE)" = "B" ]; then
        eval $LAYOUT_A
        echo "A" > "$LAYOUT_FILE"
        echo "Switched to HP Layout A: Side by side"
    else
        eval $LAYOUT_B
        echo "B" > "$LAYOUT_FILE"
        echo "Switched to HP Layout B: Laptop under monitor"
    fi
elif [[ "$DISPLAY_TYPE" == "DELL" ]]; then
    # DELL display: Use Layout C
    eval $LAYOUT_C
    echo "C" > "$LAYOUT_FILE"
    echo "Switched to DELL Layout C: Laptop on the left side under monitor with 144Hz"
fi
