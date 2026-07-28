# Windows Application Icon Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a reusable OrganizerV2 checklist brand asset and embed its multi-resolution icon into every Windows AMD64 executable produced locally, by CI, and by tagged releases.

**Architecture:** Keep the editable SVG, README PNG, and Windows ICO under `assets/branding`. Use a minimal `go-winres` JSON definition beside the `cmd/organizer` main package to generate `rsrc_windows_amd64.syso` before Windows builds; the Go linker then includes that object automatically while Linux builds ignore it. Commit source assets and configuration, but never commit the generated `.syso` file.

**Tech Stack:** Go 1.24.7, GitHub Actions, `github.com/tc-hib/go-winres@v0.3.3`, SVG, PNG, multi-resolution ICO, ImageMagick 7 for one-time raster export.

## Global Constraints

- Work only on branch `feat/windows-app-icon`.
- Use the Tabler `checklist` concept and retain its MIT attribution in `THIRD_PARTY_NOTICES.md`.
- Keep these reusable files committed: `assets/branding/organizer.svg`, `assets/branding/organizer.png`, and `assets/branding/organizer.ico`.
- The ICO must contain 16, 24, 32, 48, 64, 128, and 256 pixel images.
- Pin resource generation to `github.com/tc-hib/go-winres@v0.3.3`; never use `@latest`.
- Generate only `cmd/organizer/rsrc_windows_amd64.syso` for the current Windows target.
- Generated `.syso` files are build artifacts and must remain ignored by Git.
- Keep Linux AMD64 and Linux ARM64 build behavior unchanged.
- Do not change watcher, notification, CLI, or runtime behavior.
- Do not add an installer, tray application, Linux desktop integration, or extra logo variants.

## File Structure

### Create

- `assets/branding/organizer.svg` — editable source logo with a blue rounded-square background and white checklist glyph.
- `assets/branding/organizer.png` — 512 × 512 raster asset for README and future tooling.
- `assets/branding/organizer.ico` — multi-resolution Windows icon.
- `cmd/organizer/winres/winres.json` — input definition for the Windows resource object.
- `THIRD_PARTY_NOTICES.md` — Tabler Icons attribution and MIT license notice.

### Modify

- `.gitignore` — ignore generated Windows resource objects.
- `.github/workflows/ci.yml` — generate resources before the Windows matrix build.
- `.github/workflows/release.yml` — generate resources before the tagged Windows release build.
- `README.md` — show the logo and document reproducible branded Windows builds.

### Generated, never committed

- `cmd/organizer/rsrc_windows_amd64.syso` — COFF resource object consumed by `go build`.
- `dist/organizer-windows-amd64.exe` — local verification binary.
- `dist/extracted-winres/` — temporary resource extraction used for verification.

---

### Task 1: Add the branded source assets and attribution

**Files:**
- Create: `assets/branding/organizer.svg`
- Create: `assets/branding/organizer.png`
- Create: `assets/branding/organizer.ico`
- Create: `THIRD_PARTY_NOTICES.md`

**Interfaces:**
- Consumes: Tabler checklist geometry and MIT licensing terms.
- Produces: `assets/branding/organizer.ico`, consumed by `cmd/organizer/winres/winres.json`; `organizer.svg` and `organizer.png`, consumed by README documentation.

- [ ] **Step 1: Confirm the raster export tool is available**

Run:

```bash
magick -version
```

Expected: ImageMagick 7.x prints its version and exits successfully.

- [ ] **Step 2: Create the editable SVG**

Create `assets/branding/organizer.svg` with this exact content:

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" role="img" aria-labelledby="title desc">
  <title id="title">OrganizerV2</title>
  <desc id="desc">A white checklist on a blue rounded square.</desc>
  <rect x="1" y="1" width="22" height="22" rx="5" fill="#2563EB"/>
  <g fill="none" stroke="#FFFFFF" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M9.615 19H7a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v6"/>
    <path d="m14 18 2 2 4-4"/>
    <path d="M9 9h4"/>
    <path d="M9 13h2"/>
  </g>
</svg>
```

- [ ] **Step 3: Export the committed PNG and ICO assets**

Run from the repository root:

```bash
mkdir -p assets/branding
magick -background none assets/branding/organizer.svg -resize 512x512 assets/branding/organizer.png
magick -background none assets/branding/organizer.svg \
  -define icon:auto-resize=256,128,64,48,32,24,16 \
  assets/branding/organizer.ico
```

Expected: both files are created and are non-empty.

- [ ] **Step 4: Verify raster dimensions and ICO entries**

Run:

```bash
magick identify assets/branding/organizer.png
magick identify assets/branding/organizer.ico
```

Expected:

```text
assets/branding/organizer.png PNG 512x512
assets/branding/organizer.ico[0] ICO 256x256
assets/branding/organizer.ico[1] ICO 128x128
assets/branding/organizer.ico[2] ICO 64x64
assets/branding/organizer.ico[3] ICO 48x48
assets/branding/organizer.ico[4] ICO 32x32
assets/branding/organizer.ico[5] ICO 24x24
assets/branding/organizer.ico[6] ICO 16x16
```

The order may differ, but all seven dimensions must appear exactly once.

- [ ] **Step 5: Inspect the smallest exports**

Run:

```bash
magick assets/branding/organizer.ico[5] /tmp/organizer-24.png
magick assets/branding/organizer.ico[6] /tmp/organizer-16.png
```

Open `/tmp/organizer-24.png` and `/tmp/organizer-16.png` at native size. Expected: the clipboard outline, two list rows, and completion check remain distinguishable; no stroke touches the outer icon edge.

- [ ] **Step 6: Add the third-party notice**

Create `THIRD_PARTY_NOTICES.md`:

```markdown
# Third-Party Notices

## Tabler Icons

OrganizerV2's checklist brand asset is adapted from the `checklist` icon in Tabler Icons.

- Project: https://github.com/tabler/tabler-icons
- Copyright: Paweł Kuna and Tabler Icons contributors
- License: MIT

The original icon geometry was adapted with a project-specific background, spacing, and stroke treatment for Windows application icon legibility.
```

- [ ] **Step 7: Review only the intended files**

Run:

```bash
git status --short
git diff -- assets/branding/organizer.svg THIRD_PARTY_NOTICES.md
```

Expected: only the four files from this task are new; the PNG and ICO are binary additions.

- [ ] **Step 8: Commit the branding assets**

```bash
git add assets/branding/organizer.svg \
        assets/branding/organizer.png \
        assets/branding/organizer.ico \
        THIRD_PARTY_NOTICES.md
git commit -m "feat: add OrganizerV2 branding assets"
```

---

### Task 2: Configure reproducible Windows resource generation

**Files:**
- Create: `cmd/organizer/winres/winres.json`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: `assets/branding/organizer.ico` from Task 1.
- Produces: `cmd/organizer/rsrc_windows_amd64.syso`, consumed automatically by `go build ./cmd/organizer` when `GOOS=windows` and `GOARCH=amd64`.

- [ ] **Step 1: Demonstrate that resource generation currently fails**

Run from the repository root:

```bash
rm -f cmd/organizer/rsrc_windows_amd64.syso
(
  cd cmd/organizer
  go run github.com/tc-hib/go-winres@v0.3.3 make \
    --arch amd64 \
    --in winres/winres.json \
    --out rsrc
)
```

Expected: FAIL because `cmd/organizer/winres/winres.json` does not exist yet.

- [ ] **Step 2: Create the resource definition**

Create `cmd/organizer/winres/winres.json`:

```json
{
  "RT_GROUP_ICON": {
    "APP": {
      "0000": "../../../assets/branding/organizer.ico"
    }
  }
}
```

The icon path is relative to the directory containing `winres.json`.

- [ ] **Step 3: Ignore generated system objects**

Append this section to `.gitignore`:

```gitignore

# Generated Windows resources
*.syso
```

- [ ] **Step 4: Generate the resource object**

Run:

```bash
rm -f cmd/organizer/rsrc_windows_amd64.syso
(
  cd cmd/organizer
  go run github.com/tc-hib/go-winres@v0.3.3 make \
    --arch amd64 \
    --in winres/winres.json \
    --out rsrc
)
test -s cmd/organizer/rsrc_windows_amd64.syso
```

Expected: PASS and `cmd/organizer/rsrc_windows_amd64.syso` exists with non-zero size.

- [ ] **Step 5: Verify the generated object is ignored**

Run:

```bash
git check-ignore -v cmd/organizer/rsrc_windows_amd64.syso
git status --short
```

Expected: `.gitignore` reports the `*.syso` rule, and the generated object does not appear in `git status`.

- [ ] **Step 6: Verify Windows and Linux builds with the resource present**

Run:

```bash
mkdir -p dist
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" \
  -o dist/organizer-windows-amd64.exe ./cmd/organizer

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" \
  -o dist/organizer-linux-amd64 ./cmd/organizer

GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" \
  -o dist/organizer-linux-arm64 ./cmd/organizer
```

Expected: all three builds succeed. The Windows target includes the matching `.syso`; Linux targets ignore it because of the `_windows_amd64` suffix.

- [ ] **Step 7: Extract and verify the icon resource from the built executable**

Run:

```bash
rm -rf dist/extracted-winres
go run github.com/tc-hib/go-winres@v0.3.3 extract \
  --dir dist/extracted-winres \
  dist/organizer-windows-amd64.exe

test -s dist/extracted-winres/APP_0000.ico
magick identify dist/extracted-winres/APP_0000.ico
```

Expected: `APP_0000.ico` exists and reports the same seven icon dimensions committed in `assets/branding/organizer.ico`.

- [ ] **Step 8: Commit resource configuration**

```bash
git add cmd/organizer/winres/winres.json .gitignore
git commit -m "build: configure Windows icon resources"
```

---

### Task 3: Generate the icon in CI and tagged release builds

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: `cmd/organizer/winres/winres.json` and `assets/branding/organizer.ico` from Tasks 1–2.
- Produces: branded `organizer-windows-amd64.exe` artifacts while preserving existing Linux artifact names and release behavior.

- [ ] **Step 1: Add the Windows resource step to CI**

In `.github/workflows/ci.yml`, insert this step immediately before the existing `Build` step in the `build` job:

```yaml
      - name: Generate Windows resources
        if: matrix.goos == 'windows'
        shell: bash
        run: |
          pushd cmd/organizer
          go run github.com/tc-hib/go-winres@v0.3.3 make \
            --arch "${{ matrix.goarch }}" \
            --in winres/winres.json \
            --out rsrc
          test -s "rsrc_windows_${{ matrix.goarch }}.syso"
          popd
```

Do not alter the existing matrix, output names, build flags, or upload step.

- [ ] **Step 2: Review the CI job conditions**

Run:

```bash
grep -n -A12 -B3 "Generate Windows resources" .github/workflows/ci.yml
```

Expected: the new step appears once, before `Build`, and is guarded by `matrix.goos == 'windows'`.

- [ ] **Step 3: Add the same pinned generation step to tagged releases**

In `.github/workflows/release.yml`, insert this step immediately before the existing `Build` step in the `build` job:

```yaml
      - name: Generate Windows resources
        if: matrix.goos == 'windows'
        shell: bash
        run: |
          pushd cmd/organizer
          go run github.com/tc-hib/go-winres@v0.3.3 make \
            --arch "${{ matrix.goarch }}" \
            --in winres/winres.json \
            --out rsrc
          test -s "rsrc_windows_${{ matrix.goarch }}.syso"
          popd
```

Do not change the tag trigger, version linker flags, artifact names, or release upload list.

- [ ] **Step 4: Review the release job conditions**

Run:

```bash
grep -n -A12 -B3 "Generate Windows resources" .github/workflows/release.yml
```

Expected: the step appears once, before `Build`, and runs only for the Windows matrix entry.

- [ ] **Step 5: Validate both workflow files syntactically**

Run:

```bash
python - <<'PY'
from pathlib import Path
import yaml

for path in [Path('.github/workflows/ci.yml'), Path('.github/workflows/release.yml')]:
    with path.open('r', encoding='utf-8') as stream:
        yaml.safe_load(stream)
    print(f'valid: {path}')
PY
```

Expected:

```text
valid: .github/workflows/ci.yml
valid: .github/workflows/release.yml
```

If PyYAML is unavailable in the execution environment, use `ruby -e 'require "yaml"; YAML.load_file(ARGV[0])' <file>` for each workflow; do not add either parser as a project dependency.

- [ ] **Step 6: Reproduce the workflow build commands locally**

Run:

```bash
rm -f cmd/organizer/rsrc_windows_amd64.syso
(
  cd cmd/organizer
  go run github.com/tc-hib/go-winres@v0.3.3 make \
    --arch amd64 \
    --in winres/winres.json \
    --out rsrc
  test -s rsrc_windows_amd64.syso
)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" \
  -o dist/organizer-windows-amd64.exe ./cmd/organizer
```

Expected: resource generation and Windows build both pass.

- [ ] **Step 7: Commit workflow integration**

```bash
git add .github/workflows/ci.yml .github/workflows/release.yml
git commit -m "ci: embed icon in Windows artifacts"
```

---

### Task 4: Document branding and local Windows builds

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: committed assets and resource-generation commands from Tasks 1–3.
- Produces: user-facing project branding and exact local build instructions for PowerShell and Bash.

- [ ] **Step 1: Display the SVG near the project title**

Insert this block immediately before `# OrganizerV2`:

```html
<p align="center">
  <img src="assets/branding/organizer.svg" alt="OrganizerV2 checklist logo" width="128" height="128">
</p>
```

Keep the existing Markdown title and project description unchanged.

- [ ] **Step 2: Add a branded Windows build subsection**

Insert this subsection after the existing generic `Build from source` section and before the `Requires Go 1.22+` sentence:

````markdown
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
````

- [ ] **Step 3: Verify repository-relative links and commands**

Run:

```bash
grep -n "assets/branding/organizer.svg" README.md
grep -n "go-winres@v0.3.3" README.md
grep -n "organizer-windows-amd64.exe" README.md
```

Expected: the SVG appears once, the pinned resource tool appears in both command examples, and the Windows output filename matches CI/release naming.

- [ ] **Step 4: Execute the documented Bash commands exactly**

Run the Bash block copied from README without modification.

Expected: `dist/organizer-windows-amd64.exe` is rebuilt successfully.

- [ ] **Step 5: Commit README documentation**

```bash
git add README.md
git commit -m "docs: document branded Windows builds"
```

---

### Task 5: Run the final verification gate and prepare the pull request

**Files:**
- Verify: all files changed by Tasks 1–4
- No source file changes are expected in this task.

**Interfaces:**
- Consumes: the complete branch implementation.
- Produces: a verified branch ready for review and merge.

- [ ] **Step 1: Run static analysis and tests**

Run:

```bash
go vet ./...
go test -v -race ./...
```

Expected: both commands exit successfully.

- [ ] **Step 2: Rebuild every supported target from a clean resource state**

Run:

```bash
rm -rf dist dist/extracted-winres
rm -f cmd/organizer/rsrc_windows_amd64.syso
mkdir -p dist

(
  cd cmd/organizer
  go run github.com/tc-hib/go-winres@v0.3.3 make \
    --arch amd64 \
    --in winres/winres.json \
    --out rsrc
  test -s rsrc_windows_amd64.syso
)

GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" \
  -o dist/organizer-windows-amd64.exe ./cmd/organizer

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" \
  -o dist/organizer-linux-amd64 ./cmd/organizer

GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" \
  -o dist/organizer-linux-arm64 ./cmd/organizer
```

Expected: all three binaries exist and are non-empty.

- [ ] **Step 3: Prove the compiled executable contains the expected icon**

Run:

```bash
go run github.com/tc-hib/go-winres@v0.3.3 extract \
  --dir dist/extracted-winres \
  dist/organizer-windows-amd64.exe

test -s dist/extracted-winres/APP_0000.ico
magick identify dist/extracted-winres/APP_0000.ico
```

Expected: the extracted `APP_0000.ico` contains 16, 24, 32, 48, 64, 128, and 256 pixel entries.

- [ ] **Step 4: Confirm generated artifacts remain untracked**

Run:

```bash
git check-ignore -v cmd/organizer/rsrc_windows_amd64.syso
git check-ignore -v dist/organizer-windows-amd64.exe
git status --short
```

Expected: generated resource and binary files are ignored; the working tree is clean.

- [ ] **Step 5: Review the complete branch diff**

Run:

```bash
git log --oneline main..HEAD
git diff --stat main...HEAD
git diff --check main...HEAD
```

Expected:

- four focused implementation commits;
- no whitespace errors;
- no generated `.syso` or executable files in the diff;
- no changes to Go runtime source files.

- [ ] **Step 6: Push and open the pull request**

Run:

```bash
git push -u origin feat/windows-app-icon
gh pr create \
  --base main \
  --head feat/windows-app-icon \
  --title "feat: add branded Windows executable icon" \
  --body $'## Summary\n\n- add reusable SVG, PNG, and multi-resolution ICO branding assets\n- embed the checklist icon in Windows AMD64 CI and release binaries\n- document local branded builds and Tabler attribution\n\n## Verification\n\n- go vet ./...\n- go test -v -race ./...\n- Windows AMD64 build and resource extraction\n- Linux AMD64 and ARM64 builds'
```

Expected: GitHub returns the new pull request URL.

- [ ] **Step 7: Perform manual Windows shell verification after downloading the CI artifact**

On Windows 10 or 11:

1. Download `organizer-windows-amd64.exe` from the pull request's successful CI run.
2. Confirm Explorer displays the checklist icon.
3. Create a shortcut and confirm the shortcut displays the same icon.
4. Open file properties and confirm the icon appears in common shell surfaces.
5. Check small and large Explorer icon views.
6. If Explorer shows the previous icon, refresh the icon cache or rename the executable before judging the new build.

Expected: the custom icon is legible and consistent across Explorer, shortcut, and properties views; OrganizerV2 runtime behavior remains unchanged.
