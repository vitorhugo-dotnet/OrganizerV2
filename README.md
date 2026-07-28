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
| `notifications.actions.open_file` | Make the Windows toast body open the organized file. |
| `notifications.actions.open_location` | Show **Open Folder**, which selects the organized file in Explorer. |
| `notifications.actions.copy_path` | Show **Copy Path**. The clipboard changes only after the button is clicked. |
| `notifications.actions.move_to` | Show **Move To** when at least one valid shortcut exists. |
| `notifications.actions.copy_to` | Show **Copy To** when at least one valid shortcut exists. |
| `notifications.actions.confirm` | Show **Confirm**, which consumes the notification without changing the file. |
| `notifications.shortcuts` | Windows-only named destinations used by the **Redirect to** selection. Paths are normalized and arbitrary callback paths are rejected. |

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

Each organized file produces a native Windows toast. Windows permits at most five action buttons, so **Open File** is assigned to the notification body.

| Interaction | Description |
|---|---|
| Click the notification body | Opens the organized file with its default application. |
| **Open Folder** | Opens Explorer with the organized file selected. |
| **Copy Path** | Copies the file's final absolute path to the clipboard. |
| **Move To** | Moves the file to the destination selected in **Redirect to**. |
| **Copy To** | Copies the file to the destination selected in **Redirect to**. |
| **Confirm** | Acknowledges the notification without changing the file. |

The **Redirect to** selection and the **Move To**/**Copy To** buttons appear only when at least one valid `notifications.shortcuts` entry is configured. Shortcut names are visible in the toast, but actions carry only opaque IDs. File paths and destination directories are resolved from the running process's in-memory event registry and normalized configuration, never from callback text.

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
