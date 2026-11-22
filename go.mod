module github.com/molnarroland/WorkLogger

go 1.26.0

toolchain go1.27.4

require (
	github.com/ncruces/go-sqlite3 v0.37.0
	github.com/wailsapp/wails/v3 v3.1.0
	github.com/zalando/go-keyring v0.2.8
)

require (
	github.com/ncruces/go-sqlite3-wasm/v6 v6.5.35401 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	lukechampine.com/adiantum v1.1.1 // indirect
)

require (
	github.com/coder/websocket v1.8.14 // indirect
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/spartan-native/host-wails v0.2.0
	golang.org/x/sys v0.49.0
	golang.org/x/term v0.47.0 // indirect
)

replace github.com/spartan-native/host-wails => ./third_party/spartan-native-host-wails
