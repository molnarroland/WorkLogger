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
# SwiftPM resource bundles (the WorkLoggerKit modules' String Catalogs); the app stops if one is missing.
cp -R "$(dirname "$BIN")"/*.bundle "$APP/Contents/Resources/"
cp Support/Info.plist "$APP/Contents/Info.plist"
# Translations: compile the String Catalog into en.lproj / hu.lproj.
xcrun xcstringstool compile Sources/WorkLogger/Localizable.xcstrings --output-directory "$APP/Contents/Resources"
codesign --force --deep --sign - "$APP"

echo "Built $APP — move it to /Applications and open it."
