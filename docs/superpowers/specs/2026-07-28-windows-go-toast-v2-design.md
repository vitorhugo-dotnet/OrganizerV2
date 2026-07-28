# Design: notificações interativas do Windows com go-toast/v2

**Issue:** #8  
**Data:** 2026-07-28  
**Status:** aprovado para planejamento  
**Branch:** `feat/issue-8-go-toast-v2`

## 1. Objetivo

Migrar as notificações do Windows de `github.com/go-toast/toast` para `git.sr.ht/~jackmordaunt/go-toast/v2`, restaurando seleção nativa de destino e as ações `Move To` e `Copy To`, sem WinForms, sem script PowerShell próprio e sem alterar o comportamento das notificações Linux.

O watcher deve continuar não bloqueante. Nenhuma ação será executada antes de uma ativação explícita do usuário.

## 2. Decisão

Será usada a abordagem de **callback nativo com registro de eventos em memória**.

Cada toast receberá um identificador aleatório e opaco. O processo manterá um registro concorrente que relaciona esse ID ao caminho final do arquivo e ao estado de consumo. O callback global da biblioteca encaminhará a ativação para um handler próprio, que validará a ação, resolverá o shortcut selecionado e chamará um serviço de operações de arquivo.

O protocolo `organizerv2://` será removido após a validação do callback COM. Manter protocolo e callback como caminhos permanentes criaria duas fontes de verdade para a mesma ação.

### Alternativas rejeitadas

1. **Callback nativo com fallback URI permanente:** mantém PowerShell, registro de protocolo e duas rotas de ativação.
2. **Registro persistente de eventos:** permitiria processar toasts após reinício, mas adicionaria persistência, limpeza e sincronização fora do escopo.

## 3. Dependência e ativação

Remover:

```text
github.com/go-toast/toast
```

Adicionar:

```text
git.sr.ht/~jackmordaunt/go-toast/v2 v2.0.3
```

No startup Windows:

1. obter o executável absoluto com `os.Executable()`;
2. chamar `toast.SetAppData(...)` com:
   - `AppID`: `OrganizerV2`;
   - GUID constante e exclusivo do OrganizerV2;
   - `ActivationExe`: executável atual;
3. registrar `toast.SetActivationCallback(...)` exatamente uma vez;
4. instalar o handler ativo no dispatcher global;
5. reconhecer `-Embedding` sem iniciar a CLI ou um segundo watcher.

As ações usarão `toast.Foreground`, necessário para callback e leitura dos inputs.

### Modo `-Embedding`

O registro escolhido é apenas em memória. Portanto, um processo iniciado pelo Windows após o watcher original ter encerrado não possui o estado necessário para executar com segurança uma ação antiga.

Quando iniciado com `-Embedding`, o executável:

1. inicializará AppData, dispatcher e callback COM;
2. não iniciará watcher, Cobra ou processamento normal;
3. manterá um host de ativação por no máximo 10 segundos;
4. rejeitará silenciosamente eventos não encontrados no registro da sessão;
5. encerrará com código zero após o callback ou timeout.

Esse modo satisfaz o contrato de inicialização COM sem transportar caminhos no toast nem inventar persistência. Ações de toasts da sessão ativa funcionarão no watcher já em execução. Toasts antigos, clicados após reinício, serão ignorados com segurança e essa limitação será documentada.

### Fallback PowerShell da biblioteca

Se a biblioteca cair no fallback PowerShell:

- a notificação poderá continuar informativa;
- ações interativas serão consideradas indisponíveis;
- o erro COM será registrado de forma clara;
- nenhuma rota alternativa executará ações automaticamente;
- a organização do arquivo continuará normalmente.

## 4. Arquitetura

```text
windowsNotifier
  -> registra evento
  -> constrói toast
  -> publica sem bloquear

activationDispatcher
  -> callback global registrado uma vez
  -> encaminha para o handler ativo

WindowsNotificationActionHandler
  -> valida action ID e event ID
  -> lê seleção
  -> impede duplicidade
  -> resolve shortcut
  -> chama FileActionService

NotificationEventRegistry
  -> associa event ID ao arquivo
  -> protege concorrência
  -> controla consumo e expiração

NotificationShortcutResolver
  -> expõe shortcuts normalizados
  -> gera IDs estáveis e opacos
  -> resolve IDs contra a configuração

FileActionService
  -> Open File
  -> Open Folder
  -> Copy Path
  -> Move To
  -> Copy To
```

Tudo que depender de COM, Explorer, ShellExecute ou clipboard permanecerá sob `//go:build windows`.

## 5. Configuração

```go
type Shortcut struct {
    Name string `yaml:"name" mapstructure:"name"`
    Path string `yaml:"path" mapstructure:"path"`
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

### Normalização dos shortcuts

Durante `config.Load`:

1. aplicar `strings.TrimSpace` em nome e caminho;
2. ignorar entradas com nome ou caminho vazio;
3. expandir `~` e `~/`;
4. converter para caminho absoluto com `filepath.Abs`;
5. aplicar `filepath.Clean`;
6. manter a primeira entrada para nomes duplicados sem diferenciar maiúsculas e minúsculas;
7. manter a primeira entrada para caminhos normalizados duplicados;
8. registrar entradas ignoradas sem invalidar as demais.

No Windows, a identidade do caminho será normalizada para minúsculas. O caminho absoluto original será preservado para execução.

### IDs dos shortcuts

```text
sha256("organizerv2-shortcut-v1\x00" + caminho-normalizado-para-identidade)
```

O hash completo será codificado em hexadecimal. O toast nunca receberá o caminho como valor da seleção.

O resolver manterá um mapa somente leitura `shortcutID -> Shortcut` criado a partir da configuração normalizada. IDs desconhecidos ou removidos serão rejeitados.

### Padrões

`MoveTo` e `CopyTo` serão habilitados por padrão. Os botões só aparecerão quando houver ao menos um shortcut válido. Os shortcuts padrão serão `Desktop` e `Documents`, repetindo a experiência estabelecida no PR #3.

## 6. Modelo do toast

Cada toast terá:

- título `OrganizerV2`;
- corpo com nome final e categoria;
- `ActivationExe` absoluto;
- `eventID` opaco;
- select `destination` somente quando houver shortcut válido e `MoveTo` ou `CopyTo` habilitado;
- somente ações habilitadas.

### Argumentos

```text
v1|<actionID>|<eventID>
```

Ações aceitas:

```text
open_file
open_location
copy_path
move_to
copy_to
confirm
```

O parser exigirá exatamente três partes, versão `v1`, ação conhecida e event ID hexadecimal no tamanho esperado. Nenhuma parte será interpretada como caminho, URI ou comando.

### Seleção

O input terá ID `destination`.

Para `move_to` e `copy_to`, o handler exigirá um valor não vazio e o resolver confirmará que ele pertence à configuração atual. Para as demais ações, qualquer seleção será ignorada.

## 7. Registro e concorrência

```go
type NotificationEvent struct {
    ID          string
    CurrentPath string
    Category    string
    CreatedAt   time.Time
    ExpiresAt   time.Time
    Consumed    bool
}
```

Regras:

- `eventID` será gerado com `crypto/rand` e representado em hexadecimal;
- o caminho registrado deve ser absoluto;
- cada evento poderá produzir no máximo um efeito;
- o evento será marcado como consumido dentro do lock, antes da operação;
- falha da operação não reabrirá o evento;
- eventos expiram após sete dias;
- uma rotina leve removerá eventos expirados e será encerrada em `Close()`;
- dismiss sem ação não chamará o handler;
- eventos pertencem somente à sessão atual do processo.

O dispatcher global será registrado com `sync.Once`. O handler ativo será protegido por mutex. Testes chamarão o handler diretamente, sem depender de COM.

## 8. Handler

Ordem de processamento:

1. parsear e validar argumentos;
2. procurar o evento;
3. rejeitar evento inexistente, expirado ou consumido;
4. validar caminho absoluto e existência quando necessário;
5. validar shortcut para `move_to` e `copy_to`;
6. marcar o evento como consumido;
7. executar fora do lock;
8. registrar sucesso ou erro sem afetar o watcher.

Nenhum `panic` poderá escapar do callback. O dispatcher aplicará recuperação defensiva e logging sanitizado.

## 9. FileActionService

```go
type FileActionService interface {
    OpenFile(path string) error
    OpenLocation(path string) error
    CopyPath(path string) error
    MoveTo(path, destinationDir string) (string, error)
    CopyTo(path, destinationDir string) (string, error)
}
```

### Open File

- exigir caminho absoluto e arquivo existente;
- usar `ShellExecuteW` diretamente;
- não usar `cmd.exe`, PowerShell ou concatenação de comando.

### Open Location

- exigir caminho absoluto;
- executar `explorer.exe` com argumento separado `"/select," + path`;
- preservar espaços, Unicode, `&`, `#` e parênteses como dados do argumento.

### Copy Path

- inicializar clipboard somente quando a ação estiver habilitada;
- copiar exclusivamente no callback `copy_path`;
- registrar falha sem afetar o watcher.

### Move To

1. resolver o shortcut;
2. criar diretório com `pathutil.EnsureDir`;
3. montar destino usando o nome atual;
4. usar `pathutil.ResolveDuplicate`;
5. tentar `pathutil.MoveFile`;
6. recalcular em colisão concorrente;
7. nunca substituir arquivo existente.

### Copy To

Usará o mesmo fluxo de duplicidade, mas manterá o original. A cópia será exclusiva e falhará com `os.ErrExist` em vez de truncar o destino.

`pathutil.CopyFile` será endurecido para criar o destino com `O_CREATE|O_EXCL`. O helper não é usado atualmente pelo fluxo principal do organizer, então a alteração permanece focada nas novas ações.

Uma trava curta no serviço serializará `ResolveDuplicate + Move/Copy` entre callbacks do processo. Colisões externas continuarão sendo tratadas pela criação exclusiva e nova tentativa.

## 10. Semântica de uso único

Uma ativação válida consome o evento, inclusive `Move To`, `Copy To`, `Open File`, `Open Folder`, `Copy Path` e `Confirm`.

O caminho retornado por `Move To` será usado para testes e logging de debug. Não haverá segunda notificação nem segunda ação no mesmo toast nesta issue.

## 11. Não bloqueio e resiliência

`Notify` continuará retornando rapidamente. Registro, construção e `Push()` ocorrerão em goroutine controlada pelo notifier.

Falhas em AppData, toast, callback, shell, clipboard ou filesystem serão registradas, mas nunca interromperão o watcher nem reverterão a organização principal.

Se `Push()` falhar depois do registro, o evento será removido.

## 12. Compatibilidade Linux

- `notifier_linux.go` não terá alteração funcional;
- Linux ignorará os novos campos de configuração;
- arquivos compartilhados não importarão COM, registry ou APIs Windows;
- o comportamento atual de `Copy Path` no Linux fica fora desta issue.

## 13. Testes

### Configuração e resolver

- expansão de `~/` e caminhos relativos;
- rejeição de nome/caminho vazio;
- deduplicação determinística;
- ID estável e sem exposição do caminho;
- resolução somente de IDs configurados.

### Builder

- select somente com shortcut válido e ação dependente habilitada;
- ações desabilitadas omitidas;
- ações `Foreground`;
- event ID sem caminho;
- `ActivationExe` absoluto.

### Registro e handler

- ação desconhecida ignorada;
- payload malformado ignorado;
- evento desconhecido, expirado ou consumido ignorado;
- shortcut inexistente rejeitado;
- `Copy Path` não ocorre durante `Push()`;
- callback duplicado não repete efeito;
- notificações concorrentes permanecem associadas ao arquivo correto;
- falhas do serviço não escapam;
- `-Embedding` não inicia watcher e encerra após callback ou timeout.

### Operações de arquivo

- `Move To` cria diretório;
- `Move To` usa `arquivo (2).ext`;
- `Copy To` mantém original;
- cópia exclusiva rejeita colisão;
- arquivos existentes nunca são sobrescritos;
- caracteres especiais são passados como argumentos, não como shell.

### Build e manual

- `go test ./...`;
- `go vet ./...`;
- `GOOS=windows GOARCH=amd64 go build ./cmd/organizer`;
- `GOOS=linux GOARCH=amd64 go build ./cmd/organizer`;
- teste em Windows 10 e Windows 11;
- terminal e duplo clique;
- watcher de longa duração;
- dismiss sem efeito;
- fallback PowerShell documentado e sem ações falsas;
- toast antigo após reinício ignorado com segurança.

## 14. Documentação

`README.md` e `configs/config.yaml` explicarão:

- as seis ações;
- execução somente após clique no Windows;
- configuração dos shortcuts;
- ausência de destinos arbitrários;
- resolução de duplicidade;
- limitação do fallback PowerShell;
- necessidade do watcher da sessão permanecer ativo para resolver eventos.

## 15. Critérios de conclusão

- dependência antiga removida;
- `go-toast/v2` configurado com AppData, callback e ActivationExe;
- protocolo `organizerv2://` removido;
- select mostra apenas nomes válidos;
- caminhos não trafegam no toast;
- ações respeitam configuração;
- cada evento produz no máximo um efeito;
- nenhum arquivo existente é sobrescrito;
- watcher permanece resiliente;
- builds Windows e Linux passam;
- documentação reflete o fluxo.

## 16. Referências

- Issue #8: https://github.com/vitorhugo-dotnet/OrganizerV2/issues/8
- go-toast/v2: https://pkg.go.dev/git.sr.ht/~jackmordaunt/go-toast/v2@v2.0.3
- AppData e callback: https://pkg.go.dev/git.sr.ht/~jackmordaunt/go-toast/v2@v2.0.3/wintoast
- Microsoft, ativação de toasts para desktop: https://learn.microsoft.com/windows/apps/develop/notifications/app-notifications/toast-desktop-apps
- Schema de input: https://learn.microsoft.com/uwp/schemas/tiles/toastschema/element-input
