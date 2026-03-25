#!/bin/bash

LAYOUT_FILE="/tmp/current_display_layout"

# Paste your actual displayplacer commands below:
# e.g.:
# LAYOUT_A='displayplacer "id:XXXX res:2560x1440 hz:60 color_depth:8 scaling:on origin:(0,0) degree:0" "id:YYYY res:1800x1169 scaling:on origin:(2560,0) degree:0"'
# LAYOUT_B='displayplacer "id:XXXX res:2560x1440 hz:60 color_depth:8 scaling:on origin:(0,0) degree:0" "id:YYYY res:1800x1169 scaling:on origin:(380,1440) degree:0"'
#
# --- Paste your displayplacer commands below ---
LAYOUT_A='displayplacer "id:06821F68-21CC-4370-8CC0-BE95ACB3AC1C res:2560x1440 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0" "id:37D8832A-2D66-02CA-B9F7-8F30A301B230 res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(2560,458) degree:0"'
LAYOUT_B='displayplacer "id:06821F68-21CC-4370-8CC0-BE95ACB3AC1C res:2560x1440 hz:60 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0" "id:37D8832A-2D66-02CA-B9F7-8F30A301B230 res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(542,1440) degree:0"'

if [ ! -f "$LAYOUT_FILE" ] || [ "$(cat $LAYOUT_FILE)" = "B" ]; then
    eval $LAYOUT_A
    echo "A" > "$LAYOUT_FILE"
    echo "Switched to Layout A: Side by side"
else
    eval $LAYOUT_B
    echo "B" > "$LAYOUT_FILE"
    echo "Switched to Layout B: Laptop under monitor"
fi
