# Windows application icon design

## Status

Approved design for implementation on branch `feat/windows-app-icon`.

## Context

OrganizerV2 publishes a Windows executable but does not currently embed a project-specific application icon. The project also lacks reusable branding assets for the README, Windows shortcuts, and future packaging.

The selected visual concept is the Tabler `checklist` icon, which communicates organization, task tracking, and completed work without tying the project to a calendar-only metaphor.

## Goals

- Embed a branded icon in the Windows AMD64 executable.
- Keep reusable SVG, PNG, and ICO assets in the repository.
- Ensure CI and tagged release builds produce the same branded executable.
- Display the brand asset in the README.
- Keep Linux builds and runtime behavior unchanged.
- Make local resource generation reproducible and documented.

## Non-goals

- Creating an MSI installer.
- Adding a complete GUI or tray application.
- Redesigning Linux notifications.
- Shipping platform-specific Linux desktop files or icons.
- Creating a full visual identity system or multiple logo variants.

## Visual source and licensing

The asset will be based on `tabler:checklist` from Tabler Icons.

- Source: <https://github.com/tabler/tabler-icons>
- License: MIT
- The repository will retain the applicable copyright and license notice in `THIRD_PARTY_NOTICES.md`.

The final app icon may adjust padding, stroke weight, background, and colors for legibility at small Windows icon sizes, while preserving the checklist concept.

## Asset layout

```text
assets/
└── branding/
    ├── organizer.svg
    ├── organizer.png
    └── organizer.ico

cmd/organizer/
└── winres/
    └── winres.json
```

### Asset requirements

- `organizer.svg` is the editable source asset.
- `organizer.png` is a square, transparent or intentionally backed 512 x 512 raster export for README and future tooling.
- `organizer.ico` contains multiple image sizes suitable for Windows, including at least 16, 24, 32, 48, 64, 128, and 256 pixels.
- Small-size exports must be visually inspected so the checklist remains recognizable rather than collapsing into decorative soup.

## Windows resource generation

Use `github.com/tc-hib/go-winres` pinned to `v0.3.3`.

`go-winres make` will read `cmd/organizer/winres/winres.json` and generate a Windows-specific object file named with the target suffix, for example:

```text
cmd/organizer/rsrc_windows_amd64.syso
```

The Go toolchain automatically includes this resource when building the matching Windows target and excludes it from Linux builds because of the filename suffix.

Generated `.syso` files are build artifacts and must not be committed.

## Build flow

```text
SVG source
  -> PNG and multi-resolution ICO stored in the repository
  -> go-winres make
  -> rsrc_windows_amd64.syso generated
  -> go build ./cmd/organizer
  -> Windows executable containing the application icon
```

## CI and release changes

Both `.github/workflows/ci.yml` and `.github/workflows/release.yml` currently call `go build` directly. Their Windows jobs will be updated to:

1. Install the pinned `go-winres` version.
2. Run resource generation before `go build`.
3. Verify that the expected `.syso` file exists.
4. Build the Windows AMD64 executable normally.
5. Upload or publish the resulting executable using the existing artifact names.

Linux jobs must not install or execute `go-winres` and must retain their existing behavior.

## Local developer workflow

A documented command or small script will generate Windows resources locally before a Windows build. The command must use the pinned tool version rather than `@latest` so local and CI output do not drift.

The README will document:

- where the source assets live;
- how to regenerate the Windows resource;
- that the generated `.syso` is ignored;
- how to build the branded Windows executable.

## README presentation

The README will display `assets/branding/organizer.svg` near the project title using repository-relative markup. The image must remain useful in both light and dark GitHub themes.

## Error handling

- Resource generation failure must fail the Windows build immediately.
- Missing or invalid ICO input must not silently produce an unbranded release.
- Linux build failures must not be introduced by Windows-only tooling or files.
- No runtime fallback is required because icon embedding is a compile-time concern.

## Testing and verification

### Automated

- `go vet ./...`
- `go test ./...`
- Linux AMD64 build succeeds.
- Linux ARM64 build succeeds.
- Windows AMD64 resource generation succeeds.
- Windows AMD64 build succeeds after resource generation.
- The expected `.syso` file exists before the Windows build.

### Manual Windows verification

- Explorer displays the icon for the executable.
- A shortcut created from the executable displays the icon.
- The icon appears correctly in file properties and common shell surfaces.
- The icon remains legible at 16, 24, 32, 48, and 256 pixels.
- Clearing or refreshing the Windows icon cache is considered when validating changed builds, because Windows enjoys preserving stale icons as a historical archive nobody requested.

## Repository changes

Expected implementation files:

```text
assets/branding/organizer.svg
assets/branding/organizer.png
assets/branding/organizer.ico
cmd/organizer/winres/winres.json
.gitignore
.github/workflows/ci.yml
.github/workflows/release.yml
README.md
THIRD_PARTY_NOTICES.md
```

A helper script or `go:generate` directive may be added only if it reduces duplication without making the build harder to understand.

## Acceptance criteria

- The Windows AMD64 executable contains the OrganizerV2 checklist icon.
- CI artifacts and tagged releases both contain the icon.
- SVG, PNG, and multi-resolution ICO files are committed under `assets/branding`.
- The README displays the SVG asset.
- Tabler Icons licensing is documented.
- Resource generation uses `go-winres v0.3.3` consistently.
- Generated `.syso` files are ignored and not committed.
- Linux builds remain unchanged and pass.
- Existing OrganizerV2 runtime behavior is unaffected.
