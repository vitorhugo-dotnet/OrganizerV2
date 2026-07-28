<p align="center">
  <img src="assets/branding/organizer.svg" alt="OrganizerV2 checklist logo" width="128" height="128">
</p>

# OrganizerV2

A clean, cross-platform file organizer written in Go. Drop files into a watched folder and they are automatically sorted into category subfolders by extension.

Supports **Windows** and **Linux** from a single codebase.

---

## Features

- **Real-time watching** using fsnotify (no polling delay)
- **Extension-based classification** into configurable category folders
- **Duplicate handling** — `file (2).ext`, `file (3).ext`, …
- **Ignore incomplete downloads** — `.tmp`, `.crdownload`, `.!qB`, and more
- **Desktop notifications** — toast on Linux/Windows with interactive Windows actions
- **One-shot scan** mode with `--dry-run` preview
- **YAML configuration** — no hardcoded paths
- **CI/CD** — GitHub Actions builds and publishes release binaries

---

## Installation

### Download a binary

Grab the latest release binary for your platform from [Releases](../../releases).

### Build from source

```bash
git clone https://github.com/vitorhugo-java/organizerv2.git
cd organizerv2
go build -o organizer ./cmd/organizer
```

#### Build the branded Windows executable

The Windows icon resource is generated at build time from the committed multi-resolution ICO. The generated `.syso` file is ignored by Git.

PowerShell:

```powershell
New-Item -ItemType Directory -Force dist | Out-Null
Push-Location cmd/organizer
go run github.com/tc-hib/go-winres@v0.3.3 make --arch amd64 --in winres/winres.json --out rsrc
Pop-Location
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags="-s -w" -o dist/organizer-windows-amd64.exe ./cmd/organizer
```

Bash:

```bash
mkdir -p dist
(
  cd cmd/organizer
  go run github.com/tc-hib/go-winres@v0.3.3 make --arch amd64 --in winres/winres.json --out rsrc
)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" \
  -o dist/organizer-windows-amd64.exe ./cmd/organizer
```

Branding assets are stored under [`assets/branding`](assets/branding). The editable source is `organizer.svg`; `organizer.png` is intended for documentation and future tooling; `organizer.ico` is embedded in Windows builds.

Requires **Go 1.25+**.

---

## Quick start

```bash
# Generate a default config file
organizer config init

# Edit ~/.config/organizerv2/config.yaml to set your watch paths, then:

# Start the watcher daemon
organizer start

# Or do a one-shot scan (safe preview first)
organizer scan --dry-run ~/Downloads
organizer scan ~/Downloads
```

---

## Configuration

Config file location: `~/.config/organizerv2/config.yaml`

Use `--config /path/to/config.yaml` to override.

See [`configs/config.yaml`](configs/config.yaml) for a fully annotated example.

### Key fields

| Field | Description |
|---|---|
| `watch_paths` | Directories to watch. Each entry has `path` and `target_base`. |
| `rules` | Extension → category mappings. |
| `ignore_extensions` | Extensions that are never moved (partial downloads, temp files). |
| `fallback_category` | Destination for files with unrecognised extensions (default: `Others`). |
| `notifications.enabled` | Enable or disable desktop notifications. |
| `notifications.actions.open_file` | Show **Open File** in the Windows toast. |
| `notifications.actions.open_location` | Show **Open Location**, which selects the organized file in Explorer. |
| `notifications.actions.copy_path` | Retained for platform compatibility; it is not shown in the Python-compatible Windows toast. |
| `notifications.actions.move_to` | Add configured shortcuts to **Move file to**. Open actions move the file first when another destination is selected. |
| `notifications.actions.copy_to` | Retained for callback compatibility; no separate **Copy To** button is displayed. |
| `notifications.actions.confirm` | Show **Confirm**, which moves the file to the selected destination when it changed, then consumes the notification. |
| `notifications.shortcuts` | Windows-only named destinations listed after the file's actual current folder in **Move file to**. Paths are normalized and arbitrary callback paths are rejected. |

---

## CLI reference

```
organizer start                    Start the file watcher daemon
organizer scan [path]              One-shot scan (--dry-run to preview)
organizer config init              Write default config file
organizer config rules list        List all classification rules
organizer config rules add         Add an extension  (--category, --ext)
organizer config rules remove      Remove an extension  (--ext)
organizer version                  Print version
```

Global flags: `--config <path>`, `--log-level <level>`

---

## Notifications

### Linux

Notifications are delivered via `notify-send`. Install it with your package manager:

```bash
# Debian/Ubuntu
sudo apt install libnotify-bin

# Arch
sudo pacman -S libnotify
```

The **Copy Path** action requires one of `wl-copy` (Wayland), `xclip`, or `xsel`.

```bash
sudo apt install wl-clipboard   # Wayland
sudo apt install xclip           # X11
```

The **Open Location** action opens the folder via `xdg-open`. Linux notification behavior is otherwise unchanged.

### Windows

Each organized file produces a native Windows toast using the same interaction model as the original Python version.

| Interaction | Description |
|---|---|
| **Move file to** | The actual folder containing the organized file is selected by default. Configured shortcuts are listed after it. |
| **Open Location** | If another destination is selected, moves the file there first, then opens Explorer with the resulting file selected. |
| **Open File** | If another destination is selected, moves the file there first, then opens the resulting file with its default application. |
| **Confirm** | If another destination is selected, moves the file there and acknowledges the notification without opening it. |
| Click the notification body | No action. Explicit buttons are required. |

The toast intentionally exposes only **Open Location**, **Open File**, and **Confirm**. Shortcut names are visible in **Move file to**, but callbacks carry only opaque IDs. File paths and destination directories are resolved from the running process's in-memory event registry and normalized configuration, never from callback text.

Transfer collisions are resolved as `file (2).ext`, `file (3).ext`, and so on. Existing files are never overwritten.

The watcher must remain running for actions associated with the current session. Toasts created before an application restart no longer have a matching in-memory event and are ignored safely. If the library falls back to PowerShell, the notification may still be displayed, but interactive callbacks are unavailable.

---

## Category folders

| Category | Example extensions |
|---|---|
| Image | .jpg .png .gif .webp .heic .svg … |
| Executables | .exe .msi .deb .appimage … |
| Documents | .pdf .docx .xlsx .txt .md … |
| Compacted | .zip .rar .7z .tar.gz … |
| ISO | .iso .img .vhd … |
| Torrent | .torrent |
| Video | .mp4 .mkv .avi .mov … |
| Audio | .mp3 .flac .wav .opus … |
| Script | .py .js .go .rs .sh … |
| Others | everything else |

---

## Running as a background service

### Linux (systemd)

```ini
# ~/.config/systemd/user/organizerv2.service
[Unit]
Description=OrganizerV2 file watcher

[Service]
ExecStart=/usr/local/bin/organizer start
Restart=on-failure

[Install]
WantedBy=default.target
```

```bash
systemctl --user enable --now organizerv2
```

### Windows (startup folder)

Place a shortcut to `organizer.exe start` in:
`%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup`

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Run tests: `go test ./...`
4. Submit a pull request

---

## License

MIT — see [LICENSE](LICENSE).
