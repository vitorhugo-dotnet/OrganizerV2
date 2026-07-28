# Windows go-toast/v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrar as notificações do Windows para `go-toast/v2`, receber callbacks nativos com seleção de destino e restaurar Move To/Copy To sem sobrescrever arquivos nem bloquear o watcher.

**Architecture:** O notifier Windows registrará eventos opacos em memória, construirá toasts com callback `Foreground` e encaminhará ativações para um handler testável. Configuração, resolução de shortcuts, parsing, registro e roteamento permanecerão independentes de COM; somente builder, dispatcher, ShellExecute, Explorer, clipboard e `-Embedding` usarão arquivos com build tag Windows.

**Tech Stack:** Go 1.24.7, `git.sr.ht/~jackmordaunt/go-toast/v2 v2.0.3`, `golang.org/x/sys/windows`, `golang.design/x/clipboard`, Cobra, Viper, YAML, Windows Runtime/COM.

## Global Constraints

- Remover `github.com/go-toast/toast` e fixar `git.sr.ht/~jackmordaunt/go-toast/v2 v2.0.3`.
- Não alterar funcionalmente `internal/notifier/notifier_linux.go`.
- Não usar WinForms, script PowerShell próprio, `cmd.exe` ou valores do callback como comandos.
- Não transportar caminhos de arquivo ou de shortcut nos argumentos do toast.
- Usar action IDs exatos: `open_file`, `open_location`, `copy_path`, `move_to`, `copy_to`, `confirm`.
- Usar argumentos no formato exato `v1|<actionID>|<eventID>`.
- Usar input ID exato `destination`.
- Cada evento pode produzir no máximo um efeito.
- Eventos expiram após sete dias e pertencem somente à sessão atual do processo.
- O schema do Windows permite no máximo cinco botões: `Open File` será o clique no corpo; os botões serão Open Folder, Copy Path, Move To, Copy To e Confirm.
- Move To e Copy To nunca podem sobrescrever arquivos existentes.
- O protocolo `organizerv2://` deve ser removido depois que o callback COM estiver integrado.
- O watcher e a organização principal não podem falhar por erro de toast, callback, shell, clipboard ou filesystem de uma ação.

---

## File Map

### Existing files to modify

- `go.mod` — trocar a dependência de toast e tornar `golang.org/x/sys` dependência direta.
- `go.sum` — atualizar checksums com `go mod tidy`.
- `internal/config/config.go` — restaurar shortcuts e flags MoveTo/CopyTo, normalizar caminhos e gerar IDs.
- `internal/config/config_test.go` — cobrir defaults, YAML e normalização.
- `internal/pathutil/pathutil.go` — tornar `CopyFile` exclusivo, sem truncar destino existente.
- `internal/pathutil/pathutil_test.go` — provar preservação do destino em colisão.
- `internal/notifier/notifier_windows.go` — substituir o notifier antigo pelo runtime v2 e registro de eventos.
- `cmd/organizer/windows_launcher.go` — preservar comportamento existente e compartilhar detecção de argumentos.
- `README.md` — documentar ações, atalhos, limite de cinco botões e fallback.
- `configs/config.yaml` — restaurar `move_to`, `copy_to` e `shortcuts`.

### New shared files

- `internal/config/path_identity_windows.go` — identidade case-insensitive de caminho no Windows.
- `internal/config/path_identity_other.go` — identidade case-sensitive fora do Windows.
- `internal/config/config_windows_test.go` — deduplicação case-insensitive no Windows.
- `internal/notifier/action_protocol.go` — action IDs, encoder/parser e leitura do input.
- `internal/notifier/action_protocol_test.go` — payloads válidos, inválidos e seleção.
- `internal/notifier/shortcut_resolver.go` — lista ordenada e resolução por ID.
- `internal/notifier/shortcut_resolver_test.go` — resolução sem exposição de caminho.
- `internal/notifier/event_registry.go` — registro concorrente, claim único, expiração e limpeza.
- `internal/notifier/event_registry_test.go` — concorrência, expiração e remoção.
- `internal/notifier/action_handler.go` — validação e roteamento de callbacks.
- `internal/notifier/action_handler_test.go` — efeitos únicos, falhas e associação correta.

### New Windows-only files

- `internal/notifier/file_action_service_windows.go` — ShellExecuteW, Explorer, clipboard, Move To e Copy To.
- `internal/notifier/file_action_service_windows_test.go` — operações reais em diretório temporário e argumentos de processo.
- `internal/notifier/toast_builder_windows.go` — construir `toast.Notification` com select e limite de cinco botões.
- `internal/notifier/toast_builder_windows_test.go` — combinações de ações e corpo Open File.
- `internal/notifier/activation_dispatcher_windows.go` — AppData, callback global, handler ativo e sinal de ativação.
- `internal/notifier/activation_dispatcher_windows_test.go` — conversão de `toast.UserData`, recovery e despacho.
- `internal/notifier/embedding_windows.go` — host `-Embedding` com timeout.
- `cmd/organizer/toast_activation_windows.go` — interceptar `-Embedding` antes do Cobra.
- `cmd/organizer/toast_activation_windows_test.go` — reconhecer apenas o argumento COM esperado.

### File to delete

- `cmd/organizer/uri_handler_windows.go` — remover o fallback `organizerv2://` e o PowerShell próprio.

---

### Task 1: Migrate dependency and restore normalized notification configuration

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Create: `internal/config/path_identity_windows.go`
- Create: `internal/config/path_identity_other.go`
- Create: `internal/config/config_windows_test.go`

**Interfaces:**
- Produces: `config.Shortcut{Name, Path, ID string}`.
- Produces: `NotificationActions.MoveTo` and `NotificationActions.CopyTo`.
- Produces: normalized `NotificationConfig.Shortcuts` consumed by the notifier resolver.

- [ ] **Step 1: Write failing default and YAML tests**

Add these assertions to `TestDefault`:

```go
if !cfg.Notifications.Actions.MoveTo || !cfg.Notifications.Actions.CopyTo {
    t.Error("move_to and copy_to should be enabled by default")
}
if len(cfg.Notifications.Shortcuts) != 2 {
    t.Fatalf("expected 2 default shortcuts, got %d", len(cfg.Notifications.Shortcuts))
}
for _, shortcut := range cfg.Notifications.Shortcuts {
    if shortcut.ID == "" {
        t.Errorf("shortcut %q has empty ID", shortcut.Name)
    }
    if !filepath.IsAbs(shortcut.Path) {
        t.Errorf("shortcut %q path is not absolute: %s", shortcut.Name, shortcut.Path)
    }
}
```

Extend `TestLoadYAML` with:

```yaml
    move_to: true
    copy_to: false
  shortcuts:
    - name: " Desktop "
      path: "~/Desktop"
    - name: Desktop
      path: "~/OtherDesktop"
    - name: EmptyPath
      path: ""
    - name: Documents
      path: "./Documents"
```

Assert that the first Desktop wins, the invalid entry is ignored, relative paths become absolute, IDs are non-empty, and `copy_to` remains false.

- [ ] **Step 2: Run config tests and verify failure**

Run:

```bash
go test ./internal/config -run 'TestDefault|TestLoadYAML' -v
```

Expected: compile failure because `MoveTo`, `CopyTo`, `Shortcuts`, and `Shortcut.ID` do not exist.

- [ ] **Step 3: Add configuration structures and stable IDs**

Implement in `internal/config/config.go`:

```go
type Shortcut struct {
    Name string `yaml:"name" mapstructure:"name"`
    Path string `yaml:"path" mapstructure:"path"`
    ID   string `yaml:"-" mapstructure:"-"`
}

type NotificationActions struct {
    OpenFile     bool `yaml:"open_file" mapstructure:"open_file"`
    OpenLocation bool `yaml:"open_location" mapstructure:"open_location"`
    CopyPath     bool `yaml:"copy_path" mapstructure:"copy_path"`
    MoveTo       bool `yaml:"move_to" mapstructure:"move_to"`
    CopyTo       bool `yaml:"copy_to" mapstructure:"copy_to"`
    Confirm      bool `yaml:"confirm" mapstructure:"confirm"`
}

type NotificationConfig struct {
    Enabled   bool                `yaml:"enabled" mapstructure:"enabled"`
    Actions   NotificationActions `yaml:"actions" mapstructure:"actions"`
    Shortcuts []Shortcut          `yaml:"shortcuts" mapstructure:"shortcuts"`
}
```

Use exact defaults:

```go
MoveTo: true,
CopyTo: true,
Shortcuts: []Shortcut{
    {Name: "Desktop", Path: "~/Desktop"},
    {Name: "Documents", Path: "~/Documents"},
},
```

Add:

```go
func normalizeShortcuts(shortcuts []Shortcut) ([]Shortcut, error) {
    seenNames := make(map[string]struct{}, len(shortcuts))
    seenPaths := make(map[string]struct{}, len(shortcuts))
    result := make([]Shortcut, 0, len(shortcuts))

    for _, shortcut := range shortcuts {
        name := strings.TrimSpace(shortcut.Name)
        rawPath := strings.TrimSpace(shortcut.Path)
        if name == "" || rawPath == "" {
            continue
        }

        expanded, err := expandHome(rawPath)
        if err != nil {
            return nil, fmt.Errorf("expanding notification shortcut %q: %w", name, err)
        }
        absolute, err := filepath.Abs(expanded)
        if err != nil {
            return nil, fmt.Errorf("resolving notification shortcut %q: %w", name, err)
        }
        absolute = filepath.Clean(absolute)
        nameKey := strings.ToLower(name)
        pathKey := shortcutPathIdentity(absolute)

        if _, exists := seenNames[nameKey]; exists {
            continue
        }
        if _, exists := seenPaths[pathKey]; exists {
            continue
        }

        sum := sha256.Sum256([]byte("organizerv2-shortcut-v1\x00" + pathKey))
        shortcut = Shortcut{
            Name: name,
            Path: absolute,
            ID:   hex.EncodeToString(sum[:]),
        }
        seenNames[nameKey] = struct{}{}
        seenPaths[pathKey] = struct{}{}
        result = append(result, shortcut)
    }
    return result, nil
}
```

Call it from `normalize` after watch paths:

```go
shortcuts, err := normalizeShortcuts(cfg.Notifications.Shortcuts)
if err != nil {
    return err
}
cfg.Notifications.Shortcuts = shortcuts
```

- [ ] **Step 4: Add platform-specific path identity**

`internal/config/path_identity_windows.go`:

```go
//go:build windows

package config

import "strings"

func shortcutPathIdentity(path string) string {
    return strings.ToLower(path)
}
```

`internal/config/path_identity_other.go`:

```go
//go:build !windows

package config

func shortcutPathIdentity(path string) string {
    return path
}
```

Add `internal/config/config_windows_test.go`:

```go
//go:build windows

package config

import "testing"

func TestNormalizeShortcutsDeduplicatesWindowsPathCase(t *testing.T) {
    shortcuts, err := normalizeShortcuts([]Shortcut{
        {Name: "First", Path: `C:\Users\Hugo\Desktop`},
        {Name: "Second", Path: `c:\users\hugo\desktop`},
    })
    if err != nil {
        t.Fatal(err)
    }
    if len(shortcuts) != 1 {
        t.Fatalf("expected one shortcut, got %d", len(shortcuts))
    }
}
```

- [ ] **Step 5: Replace the dependency and tidy modules**

Run:

```bash
go get git.sr.ht/~jackmordaunt/go-toast/v2@v2.0.3
go mod edit -droprequire github.com/go-toast/toast
go mod tidy
```

Verify `go.mod` contains:

```text
git.sr.ht/~jackmordaunt/go-toast/v2 v2.0.3
golang.org/x/sys v0.33.0
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/config -v
```

Expected: PASS on the current OS.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/config
git commit -m "feat(config): restore notification shortcuts"
```

---

### Task 2: Make file copies collision-safe

**Files:**
- Modify: `internal/pathutil/pathutil.go`
- Modify: `internal/pathutil/pathutil_test.go`

**Interfaces:**
- Produces: `pathutil.CopyFile(src, dst string) error` that returns an error matching `os.ErrExist` and never truncates an existing destination.
- Consumed by: Windows `FileActionService.CopyTo` and cross-device `MoveFile` fallback.

- [ ] **Step 1: Write the failing collision test**

Add:

```go
func TestCopyFileDoesNotOverwriteExistingDestination(t *testing.T) {
    dir := t.TempDir()
    src := filepath.Join(dir, "src.txt")
    dst := filepath.Join(dir, "dst.txt")
    if err := os.WriteFile(src, []byte("new"), 0o640); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(dst, []byte("existing"), 0o600); err != nil {
        t.Fatal(err)
    }

    err := CopyFile(src, dst)
    if !errors.Is(err, os.ErrExist) {
        t.Fatalf("expected os.ErrExist, got %v", err)
    }
    got, readErr := os.ReadFile(dst)
    if readErr != nil {
        t.Fatal(readErr)
    }
    if string(got) != "existing" {
        t.Fatalf("destination was overwritten: %q", got)
    }
}
```

Add `errors` to imports.

- [ ] **Step 2: Run the test and verify failure**

```bash
go test ./internal/pathutil -run TestCopyFileDoesNotOverwriteExistingDestination -v
```

Expected: FAIL because the current `os.Create` truncates `dst`.

- [ ] **Step 3: Implement exclusive creation**

Replace the destination creation section with:

```go
info, err := in.Stat()
if err != nil {
    return fmt.Errorf("stat source: %w", err)
}
out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
if err != nil {
    return fmt.Errorf("create dest: %w", err)
}
```

Keep removal of the partially created destination on copy or close failure.

- [ ] **Step 4: Run pathutil tests**

```bash
go test ./internal/pathutil -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pathutil/pathutil.go internal/pathutil/pathutil_test.go
git commit -m "fix(pathutil): prevent copy overwrites"
```

---

### Task 3: Add action protocol and shortcut resolver

**Files:**
- Create: `internal/notifier/action_protocol.go`
- Create: `internal/notifier/action_protocol_test.go`
- Create: `internal/notifier/shortcut_resolver.go`
- Create: `internal/notifier/shortcut_resolver_test.go`

**Interfaces:**
- Produces: `notificationAction` constants.
- Produces: `encodeNotificationAction(action, eventID) string`.
- Produces: `parseNotificationAction(raw) (notificationAction, string, error)`.
- Produces: `inputValue{Key, Value string}` and `selectedDestination([]inputValue) (string, error)`.
- Produces: `notificationShortcutResolver.All()` and `.Resolve(id)`.

- [ ] **Step 1: Write protocol tests**

Use table tests covering all six actions:

```go
func TestNotificationActionRoundTrip(t *testing.T) {
    eventID := strings.Repeat("a", 64)
    actions := []notificationAction{
        actionOpenFile,
        actionOpenLocation,
        actionCopyPath,
        actionMoveTo,
        actionCopyTo,
        actionConfirm,
    }
    for _, action := range actions {
        raw := encodeNotificationAction(action, eventID)
        gotAction, gotEventID, err := parseNotificationAction(raw)
        if err != nil {
            t.Fatalf("parse %q: %v", raw, err)
        }
        if gotAction != action || gotEventID != eventID {
            t.Fatalf("got (%q, %q), want (%q, %q)", gotAction, gotEventID, action, eventID)
        }
    }
}
```

Invalid cases must include wrong version, unknown action, non-hex ID, 62-character ID, 66-character ID, missing part, and extra part.

Test destination selection:

```go
func TestSelectedDestination(t *testing.T) {
    got, err := selectedDestination([]inputValue{
        {Key: "other", Value: "ignored"},
        {Key: destinationInputID, Value: "shortcut-id"},
    })
    if err != nil || got != "shortcut-id" {
        t.Fatalf("got %q, %v", got, err)
    }
}
```

Also test missing and empty `destination` values.

- [ ] **Step 2: Run protocol tests and verify failure**

```bash
go test ./internal/notifier -run 'TestNotificationAction|TestSelectedDestination' -v
```

Expected: compile failure because the protocol types do not exist.

- [ ] **Step 3: Implement strict action protocol**

Create constants:

```go
type notificationAction string

const (
    actionOpenFile     notificationAction = "open_file"
    actionOpenLocation notificationAction = "open_location"
    actionCopyPath     notificationAction = "copy_path"
    actionMoveTo       notificationAction = "move_to"
    actionCopyTo       notificationAction = "copy_to"
    actionConfirm      notificationAction = "confirm"
    destinationInputID                    = "destination"
)
```

Implement strict parsing with `strings.Split(raw, "|")`, exactly three parts, version `v1`, a switch over the six constants, `len(eventID) == 64`, and `hex.DecodeString(eventID)`.

Define:

```go
type inputValue struct {
    Key   string
    Value string
}
```

`selectedDestination` must return an error unless exactly one non-empty `destination` value is present.

- [ ] **Step 4: Write resolver tests**

```go
func TestNotificationShortcutResolverPreservesOrderAndResolvesID(t *testing.T) {
    resolver := newNotificationShortcutResolver([]config.Shortcut{
        {ID: "id-desktop", Name: "Desktop", Path: filepath.Join(t.TempDir(), "Desktop")},
        {ID: "id-documents", Name: "Documents", Path: filepath.Join(t.TempDir(), "Documents")},
    })
    all := resolver.All()
    if len(all) != 2 || all[0].Name != "Desktop" || all[1].Name != "Documents" {
        t.Fatalf("unexpected order: %#v", all)
    }
    got, ok := resolver.Resolve("id-documents")
    if !ok || got.Name != "Documents" {
        t.Fatalf("unexpected resolve: %#v, %v", got, ok)
    }
}
```

Assert `Resolve(path)` returns false and `All()` returns a defensive copy.

- [ ] **Step 5: Implement resolver**

```go
type resolvedShortcut struct {
    ID   string
    Name string
    Path string
}

type notificationShortcutResolver struct {
    ordered []resolvedShortcut
    byID    map[string]resolvedShortcut
}
```

Only include shortcuts with non-empty `ID`, `Name`, absolute `Path`, and unique ID. Preserve configuration order. `All()` must return `append([]resolvedShortcut(nil), r.ordered...)`.

- [ ] **Step 6: Run notifier unit tests**

```bash
go test ./internal/notifier -run 'TestNotificationAction|TestSelectedDestination|TestNotificationShortcutResolver' -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/notifier/action_protocol* internal/notifier/shortcut_resolver*
git commit -m "feat(notifier): add secure action protocol"
```

---

### Task 4: Add concurrent event registry with one-time claims

**Files:**
- Create: `internal/notifier/event_registry.go`
- Create: `internal/notifier/event_registry_test.go`

**Interfaces:**
- Produces: `notificationEvent`.
- Produces: `newNotificationEventRegistry(options)`.
- Produces: `Register`, `Claim`, `Remove`, and `Close`.
- Consumed by: notifier delivery and action handler.

- [ ] **Step 1: Write registry lifecycle tests**

Define test options with deterministic dependencies:

```go
now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
registry := newNotificationEventRegistry(eventRegistryOptions{
    now:             func() time.Time { return now },
    newID:           func() (string, error) { return strings.Repeat("b", 64), nil },
    ttl:             7 * 24 * time.Hour,
    cleanupInterval: 0,
})
t.Cleanup(func() { _ = registry.Close() })
```

Cover:

- relative paths are rejected;
- registration stores absolute path/category and expiry;
- first `Claim` succeeds;
- second `Claim` fails;
- expired events fail;
- `Remove` deletes a pending event;
- `Close` is idempotent.

- [ ] **Step 2: Write concurrency test**

Launch 32 goroutines calling `Claim` for the same ID and assert exactly one success:

```go
var successes atomic.Int32
var wg sync.WaitGroup
for i := 0; i < 32; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        if _, ok := registry.Claim(event.ID); ok {
            successes.Add(1)
        }
    }()
}
wg.Wait()
if successes.Load() != 1 {
    t.Fatalf("expected one claim, got %d", successes.Load())
}
```

- [ ] **Step 3: Run tests and verify failure**

```bash
go test ./internal/notifier -run TestNotificationEventRegistry -race -v
```

Expected: compile failure because the registry does not exist.

- [ ] **Step 4: Implement registry**

Use exact structures:

```go
type notificationEvent struct {
    ID          string
    CurrentPath string
    Category    string
    CreatedAt   time.Time
    ExpiresAt   time.Time
    Consumed    bool
}

type eventRegistryOptions struct {
    now             func() time.Time
    newID           func() (string, error)
    ttl             time.Duration
    cleanupInterval time.Duration
}

type notificationEventRegistry struct {
    mu      sync.Mutex
    events  map[string]notificationEvent
    options eventRegistryOptions
    stop    chan struct{}
    done    chan struct{}
    once    sync.Once
}
```

Production defaults:

```go
now:             time.Now,
newID:           randomEventID,
ttl:             7 * 24 * time.Hour,
cleanupInterval: time.Hour,
```

Generate 32 random bytes with `crypto/rand.Read` and encode to 64 lowercase hex characters.

`Claim` must check existence, `Consumed`, and `ExpiresAt.After(now)`, set `Consumed = true` while holding the mutex, update the map, and return the claimed copy.

Only start the cleanup goroutine when `cleanupInterval > 0`; otherwise close `done` immediately so tests do not leak goroutines.

- [ ] **Step 5: Run race-enabled tests**

```bash
go test ./internal/notifier -run TestNotificationEventRegistry -race -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/notifier/event_registry*
git commit -m "feat(notifier): add one-time event registry"
```

---

### Task 5: Implement callback action handler

**Files:**
- Create: `internal/notifier/action_handler.go`
- Create: `internal/notifier/action_handler_test.go`

**Interfaces:**
- Consumes: strict action protocol, event registry, shortcut resolver.
- Produces: `fileActionService` interface.
- Produces: `windowsNotificationActionHandler.Handle(args string, data []inputValue)`.

- [ ] **Step 1: Define fake service and failing routing tests**

Use this interface:

```go
type fileActionService interface {
    OpenFile(path string) error
    OpenLocation(path string) error
    CopyPath(path string) error
    MoveTo(path, destinationDir string) (string, error)
    CopyTo(path, destinationDir string) (string, error)
}
```

Create a fake that records calls under a mutex. Add table tests for every action. Example Move To case:

```go
handler.Handle(
    encodeNotificationAction(actionMoveTo, event.ID),
    []inputValue{{Key: destinationInputID, Value: shortcut.ID}},
)
if fake.moveCalls != 1 {
    t.Fatalf("expected one MoveTo call, got %d", fake.moveCalls)
}
if fake.lastPath != event.CurrentPath || fake.lastDestination != shortcut.Path {
    t.Fatalf("unexpected arguments: %q -> %q", fake.lastPath, fake.lastDestination)
}
```

`confirm` must claim the event but produce zero service calls.

- [ ] **Step 2: Add security and resilience tests**

Cover:

- malformed payload does nothing;
- unknown action does nothing;
- unknown event does nothing;
- missing destination does not claim the event;
- unknown shortcut does not claim the event;
- duplicate callback produces only one service call;
- service error is logged and not returned/panicked;
- two concurrent events route to their own paths;
- a fake service panic is recovered by `Handle` and does not escape.

Use an injected logger:

```go
var logs []string
logf := func(format string, args ...any) {
    logs = append(logs, fmt.Sprintf(format, args...))
}
```

- [ ] **Step 3: Run tests and verify failure**

```bash
go test ./internal/notifier -run TestWindowsNotificationActionHandler -race -v
```

Expected: compile failure because handler and interface do not exist.

- [ ] **Step 4: Implement handler**

Use:

```go
type windowsNotificationActionHandler struct {
    registry  *notificationEventRegistry
    shortcuts *notificationShortcutResolver
    files     fileActionService
    logf      func(string, ...any)
}
```

Processing order:

```go
func (h *windowsNotificationActionHandler) Handle(args string, data []inputValue) {
    defer func() {
        if recovered := recover(); recovered != nil {
            h.logf("[notifier] recovered activation panic: %v", recovered)
        }
    }()

    action, eventID, err := parseNotificationAction(args)
    if err != nil {
        h.logf("[notifier] ignored invalid activation: %v", err)
        return
    }

    var destination string
    if action == actionMoveTo || action == actionCopyTo {
        shortcutID, selectErr := selectedDestination(data)
        if selectErr != nil {
            h.logf("[notifier] ignored activation without destination: %v", selectErr)
            return
        }
        shortcut, ok := h.shortcuts.Resolve(shortcutID)
        if !ok {
            h.logf("[notifier] ignored unknown shortcut ID")
            return
        }
        destination = shortcut.Path
    }

    event, ok := h.registry.Claim(eventID)
    if !ok {
        h.logf("[notifier] ignored unavailable event")
        return
    }

    var actionErr error
    switch action {
    case actionOpenFile:
        actionErr = h.files.OpenFile(event.CurrentPath)
    case actionOpenLocation:
        actionErr = h.files.OpenLocation(event.CurrentPath)
    case actionCopyPath:
        actionErr = h.files.CopyPath(event.CurrentPath)
    case actionMoveTo:
        _, actionErr = h.files.MoveTo(event.CurrentPath, destination)
    case actionCopyTo:
        _, actionErr = h.files.CopyTo(event.CurrentPath, destination)
    case actionConfirm:
        return
    }
    if actionErr != nil {
        h.logf("[notifier] activation %s failed: %v", action, actionErr)
    }
}
```

Do not log full paths at info level. Error values from lower layers must avoid embedding arbitrary callback content.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/notifier -run TestWindowsNotificationActionHandler -race -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/notifier/action_handler*
git commit -m "feat(notifier): route Windows toast activations"
```

---

### Task 6: Implement Windows file action service

**Files:**
- Create: `internal/notifier/file_action_service_windows.go`
- Create: `internal/notifier/file_action_service_windows_test.go`

**Interfaces:**
- Implements: `fileActionService`.
- Consumes: `pathutil.EnsureDir`, `ResolveDuplicate`, `MoveFile`, and exclusive `CopyFile`.

- [ ] **Step 1: Write Windows-only transfer tests**

Add `//go:build windows` to the test file.

Cover Move To duplicate:

```go
func TestWindowsFileActionServiceMoveToResolvesDuplicate(t *testing.T) {
    sourceDir := t.TempDir()
    destinationDir := t.TempDir()
    source := filepath.Join(sourceDir, "arquivo.txt")
    if err := os.WriteFile(source, []byte("new"), 0o640); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(destinationDir, "arquivo.txt"), []byte("existing"), 0o600); err != nil {
        t.Fatal(err)
    }

    service := newTestWindowsFileActionService()
    got, err := service.MoveTo(source, destinationDir)
    if err != nil {
        t.Fatal(err)
    }
    want := filepath.Join(destinationDir, "arquivo (2).txt")
    if got != want {
        t.Fatalf("got %q, want %q", got, want)
    }
    if _, err := os.Stat(source); !os.IsNotExist(err) {
        t.Fatal("source still exists")
    }
}
```

Add equivalent Copy To test asserting source remains and existing destination content remains unchanged.

- [ ] **Step 2: Write command and clipboard tests**

Inject functions:

```go
type processStarter func(name string, args ...string) error
type clipboardWriter func(text string) error
type shellOpener func(path string) error
```

Test `OpenLocation` with:

```go
path := `C:\Users\Hugo\A & B\arquivo (1) # final.txt`
```

Assert process name is exactly `explorer.exe`, there is exactly one argument, and it equals `"/select," + path`.

Test `CopyPath` calls the writer once with the exact path. Test absolute-path validation and missing-file handling for `OpenFile`.

- [ ] **Step 3: Run Windows tests and verify failure**

From Windows:

```powershell
go test ./internal/notifier -run TestWindowsFileActionService -v
```

Expected: compile failure because the service does not exist.

- [ ] **Step 4: Implement the service and ShellExecuteW**

Use:

```go
type windowsFileActionService struct {
    mu             sync.Mutex
    openFile       shellOpener
    startProcess   processStarter
    writeClipboard clipboardWriter
}
```

Production defaults:

```go
openFile:       shellExecuteOpen,
startProcess:   func(name string, args ...string) error { return exec.Command(name, args...).Start() },
writeClipboard: func(text string) error { clipboard.Write(clipboard.FmtText, []byte(text)); return nil },
```

Initialize clipboard once during service construction when Copy Path is enabled. If `clipboard.Init()` fails, store a writer that returns the initialization error; do not fail notifier construction.

Implement `shellExecuteOpen` using `shell32.dll` and `ShellExecuteW`; treat return values `<= 32` as errors. Convert operation `open` and path using `windows.UTF16PtrFromString`.

- [ ] **Step 5: Implement collision-safe transfer loop**

Use one service mutex around resolution and operation:

```go
func (s *windowsFileActionService) transfer(path, destinationDir string, move bool) (string, error) {
    if !filepath.IsAbs(path) || !filepath.IsAbs(destinationDir) {
        return "", errors.New("source and destination must be absolute")
    }
    if err := pathutil.EnsureDir(destinationDir); err != nil {
        return "", fmt.Errorf("create destination: %w", err)
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    raw := filepath.Join(destinationDir, filepath.Base(path))
    for attempt := 0; attempt < 1000; attempt++ {
        destination, err := pathutil.ResolveDuplicate(raw)
        if err != nil {
            return "", err
        }
        if move {
            err = pathutil.MoveFile(path, destination)
        } else {
            err = pathutil.CopyFile(path, destination)
        }
        if err == nil {
            return destination, nil
        }
        if !errors.Is(err, os.ErrExist) {
            return "", err
        }
    }
    return "", fmt.Errorf("could not reserve destination for %s", filepath.Base(path))
}
```

`OpenFile`, `OpenLocation`, and `CopyPath` must validate absolute paths. `OpenFile` must also require a regular existing file.

- [ ] **Step 6: Run Windows tests**

```powershell
go test ./internal/notifier -run TestWindowsFileActionService -race -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/notifier/file_action_service_windows*
git commit -m "feat(notifier): add safe Windows file actions"
```

---

### Task 7: Build the five-button toast and integrate notifier delivery

**Files:**
- Create: `internal/notifier/toast_builder_windows.go`
- Create: `internal/notifier/toast_builder_windows_test.go`
- Modify: `internal/notifier/notifier_windows.go`

**Interfaces:**
- Consumes: normalized shortcuts, registry, action handler, and file service.
- Produces: `buildWindowsToast(event, cfg, shortcuts, activationExe) toast.Notification`.
- Preserves: `Notifier.Notify` remains non-blocking.

- [ ] **Step 1: Write Windows builder tests**

Add `//go:build windows`.

With all six logical actions enabled and two shortcuts, assert:

```go
notification := buildWindowsToast(event, cfg, shortcuts, `C:\Apps\OrganizerV2\organizer.exe`)
if len(notification.Actions) != 5 {
    t.Fatalf("expected exactly five buttons, got %d", len(notification.Actions))
}
if notification.ActivationType != toast.Foreground {
    t.Fatalf("body activation type = %q", notification.ActivationType)
}
if notification.ActivationArguments != encodeNotificationAction(actionOpenFile, event.ID) {
    t.Fatalf("unexpected body activation arguments: %q", notification.ActivationArguments)
}
if len(notification.Inputs) != 1 || notification.Inputs[0].ID != destinationInputID {
    t.Fatalf("destination select missing: %#v", notification.Inputs)
}
```

Assert action contents in order:

```text
Open Folder
Copy Path
Move To
Copy To
Confirm
```

Assert Move To and Copy To use `InputID: destination` and all actions use `toast.Foreground`.

- [ ] **Step 2: Add configuration matrix tests**

Cover:

- `open_file=false` leaves body `ActivationType` and `ActivationArguments` empty;
- no shortcuts omits input, Move To, and Copy To;
- MoveTo/CopyTo disabled omits select even with shortcuts;
- every disabled button is omitted;
- no configuration creates more than five buttons;
- shortcut selection IDs are IDs, never paths;
- visible selection content is shortcut name.

- [ ] **Step 3: Run builder tests and verify failure**

```powershell
go test ./internal/notifier -run TestBuildWindowsToast -v
```

Expected: compile failure because builder does not exist.

- [ ] **Step 4: Implement builder**

Use fields from v2:

```go
notification := toast.Notification{
    AppID:         windowsToastAppID,
    Title:         "OrganizerV2",
    Body:          fmt.Sprintf("%s → %s/", filepath.Base(event.CurrentPath), event.Category),
    ActivationExe: activationExe,
}
```

When Open File is enabled:

```go
notification.ActivationType = toast.Foreground
notification.ActivationArguments = encodeNotificationAction(actionOpenFile, event.ID)
```

Build selection:

```go
notification.Inputs = append(notification.Inputs, toast.Input{
    ID:          destinationInputID,
    Title:       "Redirect to",
    Placeholder: "Choose destination",
    Selections:  selections,
})
```

Buttons must use `Content`, not the v1 `Label` field.

- [ ] **Step 5: Rewrite Windows notifier around injected Push**

Use:

```go
type toastPusher func(*toast.Notification) error

type windowsNotifier struct {
    cfg        config.NotificationConfig
    executable string
    registry   *notificationEventRegistry
    shortcuts  *notificationShortcutResolver
    handler    *windowsNotificationActionHandler
    files      *windowsFileActionService
    push       toastPusher
    closeOnce  sync.Once
}
```

`Notify`:

```go
func (n *windowsNotifier) Notify(event FileEvent) error {
    go n.deliver(event)
    return nil
}
```

`deliver` must register the final absolute destination, build the toast, call `Push`, and remove the event when Push fails. Delete the automatic clipboard write completely.

Production `push`:

```go
func(notification *toast.Notification) error {
    return notification.Push()
}
```

- [ ] **Step 6: Add notifier delivery tests**

Construct `windowsNotifier` directly with a fake pusher. Cover:

- Notify returns before a blocked pusher is released;
- Push failure removes the registered event;
- successful Push leaves the event claimable;
- Copy Path is not called during delivery;
- concurrent notifications receive distinct event IDs and correct file paths.

- [ ] **Step 7: Run Windows notifier tests**

```powershell
go test ./internal/notifier -run 'TestBuildWindowsToast|TestWindowsNotifier' -race -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/notifier/toast_builder_windows* internal/notifier/notifier_windows.go
git commit -m "feat(notifier): publish interactive Windows toasts"
```

---

### Task 8: Register COM activation, support -Embedding, and remove URI fallback

**Files:**
- Create: `internal/notifier/activation_dispatcher_windows.go`
- Create: `internal/notifier/activation_dispatcher_windows_test.go`
- Create: `internal/notifier/embedding_windows.go`
- Create: `cmd/organizer/toast_activation_windows.go`
- Create: `cmd/organizer/toast_activation_windows_test.go`
- Modify: `internal/notifier/notifier_windows.go`
- Delete: `cmd/organizer/uri_handler_windows.go`

**Interfaces:**
- Produces: `initializeWindowsActivation(executable string) error`.
- Produces: `installWindowsActivationHandler(handler)` and `clearWindowsActivationHandler(handler)`.
- Produces: `RunWindowsEmbeddingHost(timeout time.Duration)`.
- Consumes: `toast.SetAppData` and `toast.SetActivationCallback` once.

- [ ] **Step 1: Write dispatcher tests**

Test conversion:

```go
func TestDispatchWindowsActivationConvertsUserData(t *testing.T) {
    handler := &recordingActivationHandler{}
    installWindowsActivationHandler(handler)
    t.Cleanup(func() { clearWindowsActivationHandler(handler) })

    dispatchWindowsActivation("v1|confirm|"+strings.Repeat("c", 64), []toast.UserData{
        {Key: destinationInputID, Value: "shortcut-id"},
    })

    if len(handler.data) != 1 || handler.data[0].Key != destinationInputID {
        t.Fatalf("unexpected data: %#v", handler.data)
    }
}
```

Use a small interface:

```go
type activationHandler interface {
    Handle(args string, data []inputValue)
}
```

Cover no active handler, replacing handler, clearing only the same handler instance, and panic recovery.

- [ ] **Step 2: Implement global dispatcher and AppData**

Use exact constants:

```go
const (
    windowsToastAppID = "OrganizerV2"
    windowsToastGUID  = "70E4C1E0-7B44-4C34-A0D0-4361EAD57B89"
)
```

Global state:

```go
var windowsActivationState struct {
    once       sync.Once
    mu         sync.RWMutex
    handler    activationHandler
    activated  chan struct{}
    initialize error
}
```

Initialize `activated` before registering callback. `initializeWindowsActivation` must:

```go
windowsActivationState.once.Do(func() {
    windowsActivationState.activated = make(chan struct{}, 1)
    windowsActivationState.initialize = toast.SetAppData(toast.AppData{
        AppID:         windowsToastAppID,
        GUID:          windowsToastGUID,
        ActivationExe: executable,
    })
    toast.SetActivationCallback(dispatchWindowsActivation)
})
return windowsActivationState.initialize
```

`dispatchWindowsActivation` converts `[]toast.UserData` into `[]inputValue`, signals `activated` non-blockingly, and invokes the active handler outside the mutex under a recovery guard.

- [ ] **Step 3: Wire activation into notifier construction**

In `newPlatform`:

1. resolve `os.Executable()` and `filepath.Abs`;
2. create registry, resolver, service, and handler;
3. call `initializeWindowsActivation(executable)` and log errors without returning failure;
4. install the handler;
5. return the notifier.

`Close` must clear the same handler and close the registry exactly once.

- [ ] **Step 4: Write -Embedding argument test**

In `cmd/organizer/toast_activation_windows_test.go`:

```go
func TestIsToastEmbeddingInvocation(t *testing.T) {
    tests := []struct {
        args []string
        want bool
    }{
        {[]string{"organizer.exe", "-Embedding"}, true},
        {[]string{"organizer.exe", "start"}, false},
        {[]string{"organizer.exe", "--embedding"}, false},
        {[]string{"organizer.exe", "x-Embedding"}, false},
    }
    for _, tt := range tests {
        if got := isToastEmbeddingInvocation(tt.args); got != tt.want {
            t.Fatalf("args=%v got=%v want=%v", tt.args, got, tt.want)
        }
    }
}
```

- [ ] **Step 5: Implement embedding host**

`internal/notifier/embedding_windows.go`:

```go
func RunWindowsEmbeddingHost(timeout time.Duration) {
    executable, err := os.Executable()
    if err != nil {
        log.Printf("[notifier] embedding executable lookup failed: %v", err)
        return
    }
    executable, err = filepath.Abs(executable)
    if err != nil {
        log.Printf("[notifier] embedding executable normalization failed: %v", err)
        return
    }
    if err := initializeWindowsActivation(executable); err != nil {
        log.Printf("[notifier] embedding activation initialization failed: %v", err)
    }

    timer := time.NewTimer(timeout)
    defer timer.Stop()
    select {
    case <-windowsActivationState.activated:
    case <-timer.C:
    }
}
```

`cmd/organizer/toast_activation_windows.go`:

```go
//go:build windows

package main

import (
    "os"
    "time"

    "github.com/vitorhugo-java/organizerv2/internal/notifier"
)

func init() {
    if !isToastEmbeddingInvocation(os.Args) {
        return
    }
    notifier.RunWindowsEmbeddingHost(10 * time.Second)
    os.Exit(0)
}

func isToastEmbeddingInvocation(args []string) bool {
    for _, arg := range args[1:] {
        if arg == "-Embedding" {
            return true
        }
    }
    return false
}
```

- [ ] **Step 6: Delete URI handler**

Delete `cmd/organizer/uri_handler_windows.go`. Search for leftovers:

```bash
git grep -n 'organizerv2://\|ensureURIScheme\|handleURIInvocation\|powershell' -- . ':!docs/superpowers'
```

Expected: no production matches for the protocol or custom PowerShell handler.

- [ ] **Step 7: Run Windows tests and build**

```powershell
go test ./... -race
go vet ./...
go build ./cmd/organizer
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/notifier cmd/organizer
git commit -m "feat(windows): register native toast activation"
```

---

### Task 9: Update configuration and documentation

**Files:**
- Modify: `configs/config.yaml`
- Modify: `README.md`

**Interfaces:**
- Documents the exact runtime behavior delivered by Tasks 1–8.

- [ ] **Step 1: Update example configuration**

Replace the notification block with:

```yaml
notifications:
  enabled: true
  actions:
    # Clicking the notification body opens the organized file.
    open_file: true
    # Opens Explorer with the organized file selected.
    open_location: true
    # Copies the final absolute path only after clicking Copy Path.
    copy_path: true
    # Moves the file to the selected configured shortcut.
    move_to: true
    # Copies the file to the selected configured shortcut.
    copy_to: true
    # Acknowledges the notification without changing the file.
    confirm: true

  # Windows-only named destinations shown in the native "Redirect to" select.
  # Paths are normalized to absolute paths. Arbitrary destinations are rejected.
  shortcuts:
    - name: Desktop
      path: ~/Desktop
    - name: Documents
      path: ~/Documents
```

- [ ] **Step 2: Update README configuration table**

Document all six keys and add `notifications.shortcuts`. State that the toast body is Open File because Windows permits at most five buttons.

- [ ] **Step 3: Replace Windows notification section**

The section must state:

- body click: Open File;
- buttons: Open Folder, Copy Path, Move To, Copy To, Confirm;
- Move/Copy buttons and select appear only with valid shortcuts;
- Copy Path no longer runs automatically;
- file paths and destinations are resolved from in-memory state/configuration, not callback text;
- collisions produce `file (2).ext` and never overwrite;
- watcher must remain running for session events;
- old toasts after restart are ignored safely;
- PowerShell fallback may show information but cannot deliver interactive callbacks;
- Linux behavior remains unchanged.

- [ ] **Step 4: Run documentation consistency search**

```bash
git grep -n 'immediately on notification delivery\|go-toast/toast\|organizerv2://' -- README.md configs internal cmd
```

Expected: no stale behavior text or old dependency/protocol references.

- [ ] **Step 5: Commit**

```bash
git add README.md configs/config.yaml
git commit -m "docs: document interactive Windows toast actions"
```

---

### Task 10: Full verification and acceptance evidence

**Files:**
- Verify: all modified and created files.
- No new production file unless verification exposes a defect.

**Interfaces:**
- Confirms the implementation satisfies issue #8 and both design documents.

- [ ] **Step 1: Format and inspect diff**

```bash
gofmt -w internal/config internal/pathutil internal/notifier cmd/organizer
git diff --check
git status --short
git diff --stat main...HEAD
```

Expected: no formatting or whitespace errors.

- [ ] **Step 2: Run Linux validation**

```bash
go mod tidy
go vet ./...
go test ./... -race
go build ./cmd/organizer
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/organizer-linux-amd64 ./cmd/organizer
```

Expected: all commands exit zero.

- [ ] **Step 3: Run Windows validation**

On Windows PowerShell:

```powershell
go mod tidy
go vet ./...
go test ./... -race
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -o .\dist\organizer-windows-amd64.exe .\cmd\organizer
```

Expected: all commands exit zero.

- [ ] **Step 4: Run targeted race and security tests**

```bash
go test ./internal/notifier -race -count=20 -run 'TestNotificationEventRegistry|TestWindowsNotificationActionHandler'
go test ./internal/pathutil -count=20 -run 'TestCopyFileDoesNotOverwriteExistingDestination|TestResolveDuplicate'
```

Expected: all repetitions pass.

- [ ] **Step 5: Perform Windows 10/11 manual matrix**

Use a config with Desktop, Documents, and a path containing spaces and Unicode. For each organized file, verify:

```text
[ ] toast shows filename and category
[ ] clicking body opens file only when open_file=true
[ ] Open Folder selects the exact file in Explorer
[ ] Copy Path changes clipboard only after click
[ ] Move To uses selected shortcut and resolves duplicates
[ ] Copy To preserves original and resolves duplicates
[ ] Confirm changes nothing
[ ] dismiss changes nothing
[ ] paths with spaces, accents, Unicode, &, #, quotes and parentheses work
[ ] two simultaneous toasts operate on their own files
[ ] double activation executes once
[ ] terminal launch works
[ ] double-click launch works without visible console
[ ] watcher remains responsive while toast is pending
[ ] old toast after process restart is ignored safely
[ ] COM failure logs fallback limitation without stopping organization
```

- [ ] **Step 6: Verify dependency and protocol removal**

```bash
git grep -n 'github.com/go-toast/toast\|organizerv2://\|Start-Process explorer\|powershell' -- . ':!docs/superpowers'
go list -m all | grep 'go-toast'
```

Expected:

```text
git.sr.ht/~jackmordaunt/go-toast/v2 v2.0.3
```

No production matches for the old library, URI protocol, or custom PowerShell command.

- [ ] **Step 7: Commit verification-only formatting changes when present**

Only when `gofmt` or `go mod tidy` changed tracked files:

```bash
git add -A
git commit -m "chore: finalize go-toast v2 migration"
```

If no files changed, do not create an empty commit.

- [ ] **Step 8: Prepare pull request evidence**

Use this PR title:

```text
Migrate Windows notifications to go-toast/v2
```

The PR body must include:

```markdown
Closes #8

## Summary
- migrates Windows notifications to go-toast/v2 COM callbacks
- restores native destination selection, Move To and Copy To
- executes Open File, Open Folder and Copy Path only after user interaction
- removes the organizerv2:// PowerShell fallback
- preserves Linux behavior and non-blocking watcher delivery

## Validation
- [ ] go vet ./...
- [ ] go test ./... -race
- [ ] Windows amd64 build
- [ ] Linux amd64 build
- [ ] Windows 10 manual test
- [ ] Windows 11 manual test
```

---

## Plan Self-Review

### Spec coverage

- Dependency migration: Task 1.
- Shortcut normalization and stable IDs: Tasks 1 and 3.
- Exclusive copy and duplicate handling: Tasks 2 and 6.
- One-time concurrent event state: Task 4.
- Callback validation and routing: Task 5.
- Native Windows shell, Explorer and clipboard operations: Task 6.
- Native select, action visibility and five-button schema limit: Task 7.
- AppData, callback registration, ActivationExe and `-Embedding`: Task 8.
- Removal of URI/PowerShell fallback: Task 8.
- README and example config: Task 9.
- Linux/Windows builds, race tests and manual acceptance: Task 10.

### Type consistency

- `notificationAction`, `inputValue`, `notificationEvent`, `resolvedShortcut`, `fileActionService`, and `activationHandler` are defined before consumers.
- Action IDs, destination input ID, event ID length and AppData GUID are constant across tasks.
- Open File is consistently represented by `Notification.ActivationArguments`; all other logical actions are buttons.
- `MoveTo` and `CopyTo` consistently return the final destination path and an error.

### Scope

The plan changes only Windows notification behavior, shared configuration needed by it, and collision safety required by Move/Copy. It does not redesign Linux notifications, add persistent history, add arbitrary destinations, or refactor the organizer move pipeline.
