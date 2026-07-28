# Design: notificações interativas do Windows com go-toast/v2

**Issue:** #8  
**Data:** 2026-07-28  
**Status:** aprovado para planejamento  
**Branch:** `feat/issue-8-go-toast-v2`

## 1. Objetivo

Migrar as notificações do Windows de `github.com/go-toast/toast` para `git.sr.ht/~jackmordaunt/go-toast/v2`, restaurando seleção nativa de destino e as ações `Move To` e `Copy To`, sem WinForms, sem script PowerShell próprio e sem alterar o comportamento das notificações Linux.

A implementação deve preservar o watcher não bloqueante e executar qualquer ação somente depois de uma ativação explícita do usuário.

## 2. Decisão

Será usada a abordagem de **callback nativo com estado de eventos em memória**.

Cada toast receberá um identificador aleatório e opaco. O processo manterá um registro concorrente que relaciona esse identificador ao caminho final do arquivo, à categoria e ao estado de consumo. O callback global da biblioteca encaminhará a ativação para um handler próprio, que validará a ação, resolverá o shortcut selecionado e chamará um serviço de operações de arquivo.

O protocolo `organizerv2://` será removido quando o callback COM estiver validado, pois manter protocolo e callback como caminhos paralelos criaria duas fontes de verdade para a mesma ação.

### Alternativas rejeitadas

1. **Callback nativo com fallback URI permanente:** reduz parte do risco inicial, mas mantém PowerShell, registro de protocolo e duas rotas de ativação.
2. **Registro persistente de eventos:** permitiria processar toasts antigos após reinício do processo, mas adicionaria persistência, limpeza e sincronização fora do escopo atual.

## 3. Dependência e ativação do Windows

A dependência antiga será removida e a implementação será fixada em `git.sr.ht/~jackmordaunt/go-toast/v2 v2.0.3`, versão estável da linha v2 verificada durante o design.

No startup Windows:

1. obter o caminho absoluto do executável atual com `os.Executable()`;
2. chamar `toast.SetAppData(...)` com:
   - `AppID`: `OrganizerV2`;
   - GUID constante e próprio do OrganizerV2;
   - `ActivationExe`: executável atual;
3. registrar `toast.SetActivationCallback(...)` exatamente uma vez;
4. instalar o handler ativo no dispatcher global do pacote;
5. reconhecer `-Embedding` sem iniciar um segundo watcher nem executar o fluxo normal da CLI.

As ações do toast usarão `toast.Foreground`, pois essa ativação é necessária para receber o callback e os valores dos inputs.

O callback será a implementação principal. Se a biblioteca cair no fallback PowerShell, a notificação poderá continuar informativa, mas as ações interativas serão consideradas indisponíveis e isso será registrado de forma clara. O fallback nunca executará uma versão alternativa das ações por conta própria.

## 4. Arquitetura

```text
windowsNotifier
  -> registra o evento
  -> constrói o toast
  -> publica sem bloquear o watcher

activationDispatcher
  -> callback global registrado uma vez
  -> encaminha para o handler Windows ativo

WindowsNotificationActionHandler
  -> valida action ID e event ID
  -> lê a seleção do input
  -> impede ativação duplicada
  -> resolve o shortcut
  -> chama FileActionService

NotificationEventRegistry
  -> associa event ID ao arquivo correto
  -> protege acesso concorrente
  -> controla consumo e expiração

NotificationShortcutResolver
  -> expõe somente shortcuts normalizados
  -> gera IDs estáveis e opacos
  -> resolve IDs novamente contra a configuração

FileActionService
  -> Open File
  -> Open Folder
  -> Copy Path
  -> Move To
  -> Copy To
```

As implementações que dependem de COM, Explorer, ShellExecute e clipboard continuarão protegidas por `//go:build windows`.

## 5. Configuração

### Estruturas

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

### Normalização de shortcuts

Durante `config.Load`:

1. aplicar `strings.TrimSpace` em nome e caminho;
2. rejeitar entradas com nome ou caminho vazio;
3. expandir `~` e `~/`;
4. converter para caminho absoluto com `filepath.Abs`;
5. limpar com `filepath.Clean`;
6. manter a primeira entrada quando houver nome duplicado sem diferenciar maiúsculas e minúsculas;
7. manter a primeira entrada quando houver caminho normalizado duplicado;
8. registrar entradas ignoradas sem interromper o carregamento das demais.

No Windows, a chave usada para comparar caminhos será normalizada para minúsculas antes da deduplicação e da geração do ID. O caminho preservado para execução continuará com sua forma absoluta original.

### IDs de shortcut

O ID será:

```text
sha256("organizerv2-shortcut-v1\x00" + caminho-normalizado-para-identidade)
```

O valor será codificado em hexadecimal completo. O caminho não será incluído nos argumentos da ação ou no valor visível do select.

O resolver manterá um mapa somente leitura `shortcutID -> Shortcut` criado a partir da configuração já normalizada. Um ID desconhecido, removido ou alterado será rejeitado.

### Configuração padrão

`MoveTo` e `CopyTo` serão habilitados por padrão, mas os botões somente aparecerão quando existir ao menos um shortcut válido. Os atalhos padrão serão `Desktop` e `Documents`, repetindo a experiência já estabelecida no PR #3.

## 6. Modelo do toast

Cada notificação terá:

- título `OrganizerV2`;
- corpo com nome final do arquivo e categoria;
- `ActivationExe` apontando para o executável atual;
- um `eventID` opaco;
- input de seleção `destination` apenas quando houver shortcuts válidos e ao menos uma das ações `MoveTo` ou `CopyTo` estiver habilitada;
- somente ações habilitadas pela configuração.

### Argumentos das ações

Os argumentos serão codificados internamente em formato simples e estrito:

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

O parser exigirá exatamente três partes, versão `v1`, action ID conhecido e event ID no formato esperado. Nenhuma parte será interpretada como comando, URI ou caminho.

### Seleção

O input terá ID `destination`. Para `Move To` e `Copy To`, o handler exigirá exatamente um valor não vazio e o resolver confirmará que ele corresponde a um shortcut da configuração atual.

Para as demais ações, qualquer seleção presente será ignorada.

## 7. Registro de eventos e concorrência

`NotificationEventRegistry` usará `sync.Mutex` e armazenará:

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

### Regras

- `eventID` será gerado com `crypto/rand` e representado em hexadecimal.
- O caminho registrado deve ser absoluto.
- Cada ativação válida poderá consumir o evento uma única vez.
- O evento será marcado como consumido dentro da seção crítica, antes do efeito colateral.
- Se a operação falhar, o evento continuará consumido. Isso evita que uma entrega duplicada repita uma operação parcialmente executada.
- Eventos expiram após sete dias.
- Um goroutine leve removerá eventos expirados periodicamente e será encerrado em `Close()`.
- Fechar ou dispensar o toast sem ação não chamará o callback de negócio e não consumirá o evento.
- Eventos pertencem apenas à sessão atual do processo. Um toast antigo ativado depois que o processo e seu registro forem reiniciados será ignorado com segurança.

O dispatcher global será registrado com `sync.Once`. Ele encaminhará o callback para o handler ativo protegido por mutex. Testes chamarão o handler diretamente, evitando dependência de COM.

## 8. Handler de ações

O handler seguirá esta ordem:

1. parsear e validar argumentos;
2. procurar o evento;
3. rejeitar evento inexistente, expirado ou consumido;
4. validar o caminho absoluto e a existência do arquivo quando a ação exigir;
5. validar shortcut para `move_to` e `copy_to`;
6. marcar o evento como consumido;
7. executar a ação fora do lock do registro;
8. registrar sucesso ou erro sem derrubar o watcher.

Nenhum `panic` originado no callback deverá escapar. O dispatcher aplicará recuperação defensiva e logging sanitizado.

## 9. FileActionService

O serviço terá dependências pequenas e substituíveis em testes:

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

- confirmar que o caminho é absoluto e aponta para arquivo existente;
- usar `ShellExecuteW` diretamente, sem `cmd.exe`, PowerShell ou concatenação de comando;
- retornar erro sanitizado se o arquivo não existir ou a chamada falhar.

### Open Location

- confirmar caminho absoluto;
- executar `explorer.exe` com argumento separado `"/select," + path`;
- não usar shell;
- preservar espaços, Unicode, `&`, `#` e parênteses como dados do argumento.

### Copy Path

- inicializar clipboard no startup somente se a ação estiver habilitada;
- copiar apenas dentro do callback `copy_path`;
- uma falha de clipboard será registrada e não propagará para o watcher.

### Move To

1. validar o shortcut resolvido;
2. criar o diretório com `pathutil.EnsureDir`;
3. montar o destino com o nome atual do arquivo;
4. usar `pathutil.ResolveDuplicate`;
5. tentar `pathutil.MoveFile`;
6. em colisão criada entre resolução e operação, recalcular e repetir até o limite existente;
7. nunca substituir arquivo existente.

### Copy To

Seguirá o mesmo fluxo de resolução de duplicidade, usando uma cópia exclusiva que falha com `os.ErrExist` em vez de truncar o destino. O original será mantido.

Como endurecimento necessário para o critério “nunca sobrescrever”, `pathutil.CopyFile` passará a criar o destino com `O_CREATE|O_EXCL`. Esse helper não é usado pelo fluxo atual do organizer, portanto a mudança permanece focada na segurança das novas ações.

Uma trava curta no serviço serializará a sequência `ResolveDuplicate + Move/Copy` entre callbacks do processo. Colisões externas ainda serão tratadas por tentativa exclusiva e novo nome.

## 10. Comportamento após Move To

Embora o evento mantenha `CurrentPath`, a ativação é de uso único. Portanto, uma ação `Move To` bem-sucedida não permitirá uma segunda ação no mesmo toast.

O caminho retornado será usado apenas para logging de debug e para facilitar testes. Não será criada uma segunda notificação nesta issue.

Essa decisão mantém a regra de execução exatamente uma vez e evita uma máquina de estados desnecessária dentro da Central de Notificações.

## 11. Não bloqueio e resiliência

`Notify` continuará retornando rapidamente. A construção, o registro do evento e o `Push()` ocorrerão em goroutine controlado pelo notifier.

Falhas em:

- registro de AppData;
- publicação do toast;
- callback;
- shell;
- clipboard;
- filesystem;

serão registradas, mas nunca interromperão o watcher nem reverterão a organização principal do arquivo.

Se o registro do evento ocorrer e `Push()` falhar, o evento será removido imediatamente.

## 12. Compatibilidade Linux

- `notifier_linux.go` não será alterado funcionalmente.
- Estruturas compartilhadas de configuração receberão os novos campos, mas o Linux os ignorará.
- Nenhum pacote COM, registry ou API Windows será importado por arquivos sem build tag.
- `go test ./...` e build Linux continuarão fazendo parte da validação obrigatória.

O comportamento atualmente existente de `Copy Path` no Linux não será redesenhado nesta issue.

## 13. Testes

### Configuração e resolver

- normaliza `~/` e caminhos relativos;
- rejeita nome e caminho vazios;
- deduplica nomes e caminhos de forma determinística;
- gera ID estável para o mesmo caminho;
- não expõe o caminho no ID;
- resolve somente IDs existentes.

### Builder do toast

- inclui select somente quando houver shortcut válido e ação dependente habilitada;
- omite ações desabilitadas;
- usa `Foreground`;
- inclui event ID sem incluir caminho;
- inclui `ActivationExe` absoluto.

### Registro e handler

- ignora action ID desconhecido;
- ignora payload malformado;
- ignora evento desconhecido, expirado ou consumido;
- rejeita shortcut inexistente;
- `Copy Path` não ocorre durante `Push()`;
- callback duplicado executa zero vezes após a primeira ativação;
- notificações concorrentes continuam associadas ao arquivo correto;
- falhas do serviço não escapam do callback.

### Operações de arquivo

- `Move To` cria diretório quando necessário;
- `Move To` usa `arquivo (2).ext` e não sobrescreve;
- `Copy To` mantém o original;
- `Copy To` usa resolução de duplicidade;
- cópia exclusiva rejeita colisão;
- caminhos com espaços, acentos, Unicode, `&`, `#` e parênteses são passados como argumentos, não como shell.

### Build e validação manual

- `go test ./...`;
- `go vet ./...`;
- `GOOS=windows GOARCH=amd64 go build ./cmd/organizer`;
- `GOOS=linux GOARCH=amd64 go build ./cmd/organizer`;
- teste manual em Windows 10 e Windows 11;
- execução pelo terminal e por duplo clique;
- ativação com watcher rodando por longo período;
- fechamento normal sem efeito colateral;
- validação de `-Embedding` sem iniciar watcher duplicado;
- confirmação de degradação informativa quando COM falhar e o fallback PowerShell entrar em uso.

## 14. Documentação

`README.md` e `configs/config.yaml` serão atualizados para explicar:

- as seis ações disponíveis;
- que ações são executadas somente após clique no Windows;
- a configuração de shortcuts;
- a ausência de destinos arbitrários;
- o comportamento de resolução de duplicidade;
- a limitação do fallback PowerShell;
- a necessidade de manter o watcher ativo para que eventos da sessão atual sejam resolvidos.

## 15. Critérios de conclusão do design

A implementação será considerada aderente quando:

- a dependência antiga estiver removida;
- `go-toast/v2` estiver configurado com AppData, callback global e ActivationExe;
- o protocolo `organizerv2://` não for mais necessário;
- o select mostrar somente nomes de shortcuts válidos;
- caminhos não forem transportados pelo toast;
- todas as ações obedecerem à configuração;
- cada evento puder produzir no máximo um efeito;
- arquivos existentes nunca forem sobrescritos;
- falhas de notificação não afetarem o watcher;
- builds Windows e Linux passarem;
- documentação e configuração de exemplo refletirem o novo fluxo.

## 16. Referências

- Issue #8: https://github.com/vitorhugo-dotnet/OrganizerV2/issues/8
- go-toast/v2: https://pkg.go.dev/git.sr.ht/~jackmordaunt/go-toast/v2@v2.0.3
- AppData e callback: https://pkg.go.dev/git.sr.ht/~jackmordaunt/go-toast/v2@v2.0.3/wintoast
- Microsoft, ativação de toasts para desktop: https://learn.microsoft.com/windows/apps/develop/notifications/app-notifications/toast-desktop-apps
- Schema de input: https://learn.microsoft.com/uwp/schemas/tiles/toastschema/element-input
