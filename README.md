# ConanDirectConnect

Small Windows helper for launching Conan Exiles into a specific server.

It writes Conan Exiles' restart-ticket state, updates the relevant client
`Game.ini` values, then asks Steam to start app `440900` through
`steam://run/440900`. This keeps Steam and BattlEye in the normal launch path
while allowing the game to consume the restart ticket for automatic server
entry.

## Download

Download the latest `ConanDirectConnect-*-windows-amd64.zip` from GitHub
Releases, unzip it, then run `ConanDirectConnect.exe`.

The release artifact is intentionally a zip file instead of a bare `.exe`.

## Usage

Pass exactly one address selector:

```powershell
.\ConanDirectConnect.exe -ip 203.0.113.10
.\ConanDirectConnect.exe -domain example.com
```

Common options:

```powershell
.\ConanDirectConnect.exe -ip 203.0.113.10 -port 7777
.\ConanDirectConnect.exe -domain conan.example.com -game-dir "D:\SteamLibrary\steamapps\common\Conan Exiles"
.\ConanDirectConnect.exe -ip 203.0.113.10 -no-launch
```

`-ip` requires an IPv4 address and skips DNS lookup. `-domain` resolves the
domain to IPv4 before writing the restart ticket, because Conan direct connect
expects a numeric `IP:port` target.

`-no-launch` writes the Conan client state without starting Steam. Use it to
inspect `ModRestartData.json` and `Game.ini` before a real launch.

## Mod Boundary

This helper does not manage mods.

It does not query A2S rules, does not accept a Workshop ID list, does not
generate `ConanSandbox/servermodlist.txt`, and does not inspect or modify
`ConanSandbox/Mods/modlist.txt`. Mod subscription, download, ordering, and
mismatch handling remain the responsibility of the official launcher and the
game.

The only mod-related restart-ticket field it writes is `ModList`, pointing to
the existing `ConanSandbox/servermodlist.txt`. If that file is missing, the
helper fails instead of guessing a mod list.

## What It Changes

Before launch, the helper:

1. Locates the Conan Exiles Steam install.
2. Backs up `ConanSandbox\Saved\Config\Windows\Game.ini`.
3. Sets `SavedServers.LastConnected` and `SavedServers.LastPassword`.
4. Enables `Settings.ModMismatch.AutoSubscribe`, `AutoConnect`, and
   `bAutoRestart`.
5. Writes `ConanSandbox\Saved\ModRestartData.json`.
6. Starts Steam with `steam://run/440900`.

Each run appends diagnostics to `ConanDirectConnect.log` beside the executable.

## Build

Requirements:

- Go 1.22 or newer
- `zip` and `unzip` when building the release artifact on macOS or Linux

Build the Windows executable:

```bash
GOOS=windows GOARCH=amd64 go build -o ConanDirectConnect.exe .
```

Build the release zip:

```bash
VERSION=v0.1.0 ./scripts/build-release.sh
```

The zip is written to `dist/`.

## Notes

The default launch mode is `steam-run`. `continue-session` and
`launcher-connect` are retained as diagnostic modes, but Steam-run is the
validated path.
