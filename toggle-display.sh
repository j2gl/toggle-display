#!/bin/sh
# Compatibility entry point. Build once with `make build`, or use the Go
# run fallback while developing.
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if [ -n "${TOGGLE_DISPLAY_BIN:-}" ] && [ -x "$TOGGLE_DISPLAY_BIN" ]; then
    exec "$TOGGLE_DISPLAY_BIN" "$@"
fi

for candidate in \
    "$SCRIPT_DIR/toggle-display" \
    "$SCRIPT_DIR/bin/toggle-display" \
    "${HOME:-}/.local/bin/toggle-display"
do
    if [ -x "$candidate" ]; then
        exec "$candidate" "$@"
    fi
done

if command -v go >/dev/null 2>&1; then
    # Build a short-lived fallback so the wrapper preserves the Go program's
    # exit status even when the user has not run `make build` yet.
    TEMP_BINARY=$(mktemp "${TMPDIR:-/tmp}/toggle-display.XXXXXX")
    rm -f "$TEMP_BINARY"
    cleanup() { rm -f "$TEMP_BINARY"; }
    trap cleanup EXIT
    if ! (CDPATH= cd -- "$SCRIPT_DIR" && go build -o "$TEMP_BINARY" ./cmd/toggle-display); then
        echo "Error: failed to build toggle-display." >&2
        exit 1
    fi
    if "$TEMP_BINARY" "$@"; then
        status=0
    else
        status=$?
    fi
    exit "$status"
fi

echo "Error: toggle-display binary not found and Go is not installed." >&2
echo "Build it with: make build" >&2
exit 1
