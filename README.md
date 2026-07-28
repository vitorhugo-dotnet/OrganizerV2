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
- **Desktop notifications** — toast on Linux/Windows with action buttons
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

Requires **Go 1.22+**.

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
| `notifications.enabled` | Enable/disable desktop notifications. |
| `notifications.actions` | Toggle individual notification buttons (`open_file`, `open_location`, `copy_path`, `confirm`). |

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

The **Open Location** action opens the folder via `xdg-open`.

### Windows

Each file move triggers a native Windows toast notification with action buttons:

| Button | Description |
|---|---|
| **Open File** | Opens the moved file with its default application. |
| **Open Folder** | Opens the destination folder in Explorer. |
| **OK** | Dismisses the toast without any additional action. |

The **Copy Path** action writes the absolute destination path to the Windows clipboard immediately on notification delivery (no button click required).

Each button can be individually enabled or disabled via `notifications.actions` in the config file.

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
