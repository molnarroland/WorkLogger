#!/bin/bash
# Builds build/WorkLogger.app — a real app bundle, so macOS remembers the calendar permission
# and shows "WorkLogger" (not Terminal) in the prompt and in Privacy settings.
set -euo pipefail
cd "$(dirname "$0")/.."

swift build -c release
BIN="$(swift build -c release --show-bin-path)/WorkLogger"
APP="build/WorkLogger.app"

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BIN" "$APP/Contents/MacOS/WorkLogger"
cp Support/Info.plist "$APP/Contents/Info.plist"
codesign --force --deep --sign - "$APP"

echo "Built $APP — move it to /Applications and open it."
