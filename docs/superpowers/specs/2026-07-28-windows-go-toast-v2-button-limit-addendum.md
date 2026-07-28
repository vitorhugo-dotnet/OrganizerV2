# Addendum: limite de ações do toast do Windows

**Design principal:** `2026-07-28-windows-go-toast-v2-design.md`  
**Issue:** #8  
**Data:** 2026-07-28  
**Status:** aprovado por restrição da plataforma

## Restrição descoberta

O schema oficial de notificações do Windows permite no máximo cinco elementos `action` dentro de um toast. A configuração da issue possui seis ações lógicas:

1. Open File
2. Open Folder
3. Copy Path
4. Move To
5. Copy To
6. Confirm

Não é possível renderizar seis botões nativos no mesmo toast sem violar o schema. Itens de menu de contexto também compartilham o mesmo limite combinado e não resolvem o problema.

## Decisão

`Open File` será associado ao clique no corpo da notificação por meio de:

```go
ActivationType:      toast.Foreground,
ActivationArguments: encodeAction(actionOpenFile, eventID),
```

Os cinco botões disponíveis serão, quando habilitados:

1. Open Folder
2. Copy Path
3. Move To
4. Copy To
5. Confirm

Todos usarão `toast.Foreground` e argumentos opacos no formato definido no design principal.

## Regras

- Se `open_file` estiver habilitado, clicar no corpo executará `Open File`.
- Se `open_file` estiver desabilitado, o corpo não terá ativação de negócio; clicar nele apenas fechará a notificação.
- `Open File` continuará executando somente após interação explícita.
- A configuração das seis ações permanece inalterada.
- Botões desabilitados serão omitidos.
- O número de botões nunca poderá exceder cinco.
- `Confirm` continua sendo um botão explícito sem efeitos colaterais.
- Fechar ou dispensar a notificação sem clicar em ação continua sem efeitos colaterais.

## Testes adicionais

- builder com todas as ações habilitadas produz exatamente cinco botões;
- o corpo recebe `open_file` somente quando a ação está habilitada;
- desabilitar `open_file` remove `ActivationType` e `ActivationArguments` do corpo;
- nenhum arranjo de configuração produz mais de cinco botões;
- os cinco botões preservam seus action IDs corretos.

## Referências

- Microsoft Toast XML schema, `actions`: https://learn.microsoft.com/en-us/uwp/schemas/tiles/toastschema/element-actions
- Microsoft Toast schema root: https://learn.microsoft.com/en-us/uwp/schemas/tiles/toastschema/schema-root
