# Agent Kit – Guia de Uso

Este projeto fornece um kit para rodar **Agentes GPT modulares em Go**, com suporte a:
- Execução simples de prompts
- Cache de contexto (prompt prefixado)
- Roteamento dinâmico de prompts (router)
- Integração com ferramentas (atualmente **Postgres** e **Postgres Embedding**)

---

## Pré-requisitos

1. **Go 1.22+**
2. **Banco PostgreSQL** com extensão `pgvector` instalada.
3. **Variáveis de ambiente**:
   ```bash
   export OPENAI_API_KEY="sua_api_key"
   export SUPRABASE_PGSQL="postgres://user:pass@host:5432/db"
   export GPT_MODEL="gpt-4.1"
   export EMBEDDING_MODEL="text-embedding-3-small"
   export DB_SCHEMA="suporte"
   export EMBEDDING_DIM="1536"
   export TOOLS_PATH="tools.yml"   # opcional (default: tools.yml)
   ```

---

## Estrutura de Prompts

### Sem Router (mais simples)
Se você for rodar **sem router**, basta ter **um único arquivo** de instruções, com qualquer nome:
```
prompt/instrucoes.md
```

E no `main.go`, você aponta direto para este arquivo.

---

### Com Router
Se você quiser usar **router**, é necessário organizar os arquivos assim:

```
prompt/
 └── suporte/
     ├── suporte.md      # prompt base
     ├── router.md       # (obrigatório para rotear) arquivo de roteamento
     ├── tecnico.md      # prompt especializado 1
     ├── financeiro.md   # prompt especializado 2
     └── geral.md        # fallback
```

- `router.md` contém as regras de roteamento.
- Os outros arquivos (`tecnico.md`, `geral.md`, etc.) são **candidatos** que o LLM pode escolher. (podem existir N prompts especializados, desde que eles sejam definidos no router.md)
- O `suporte.md` é o **prompt base** que sempre será aplicado. (Personalidade da IA)

---

## Como usar

### 1. Agent Simples (sem cache, sem router, sem tools)

Esse fluxo roda apenas com o **prompt base** + **mensagem do usuário**.

```go
out, err := agent.Run(
    context.Background(),
    cli,            // cliente OpenAI
    model,          // ex: "gpt-4.1"
    embModel,       // ex: "text-embedding-3-small"
    sessionID,      // id único do usuário
    "prompt/instrucoes.md", // caminho do prompt base
    "Olá, preciso de ajuda!",    // mensagem do usuário
    false,           // verbose
)
```

---

### 2. Agent com Prompt em Cache

Aqui ativamos o **cache de contexto** (`WithCachedContext`), útil para reaproveitar prompts grandes.

```go
out, err := agent.Run(
    context.Background(),
    cli,
    model,
    embModel,
    sessionID,
    "prompt/instrucoes.md",
    "Quero saber como abrir um chamado",
    false,
    agent.WithCachedContext("prompt/instrucoes.md"),
)
```

> O cache evita reenvio completo do prompt fixo a cada request. Usar em prompts com muitas linhas

---

### 3. Agent com Router

O **router** permite escolher dinamicamente entre múltiplos prompts especializados.

```go
out, err := agent.RouteAndRun(
    context.Background(),
    cli,
    model,
    embModel,
    sessionID,
    "prompt/suporte/suporte.md",  // prompt base
    "Não consigo criar usuário no sistema",
    "prompt/suporte/router.md",   // router ativo
    true, // verbose (JSON detalhado)
)
```

- O `router.md` instrui o LLM a escolher entre candidatos no mesmo diretório (`tecnico.md`, `geral.md`, etc.).
- O prompt escolhido é usado junto com o `basePrompt`.

---

## Agent com Tools

O agente pode chamar **Tools** quando a resposta começa com `TOOL:`.  
Atualmente há suporte para:

- **Postgres** → queries SQL diretas (parametrizadas)  
- **Postgres Embedding** → busca semântica usando `pgvector`

### Configuração (`tools.yml`)

Exemplo de tools disponíveis:

```yaml
tools:
  - name: db_invoice
    description: "Consulta status de uma fatura pelo ID"
    type: postgres
    conn: "ENV:SUPRABASE_PGSQL"
    query_template: "SELECT status, due_date FROM table_invoices WHERE invoice_id = $1::int"

  - name: search_docs
    description: "Busca semântica em documentação"
    type: postgres_embedding
    conn: "ENV:SUPRABASE_PGSQL"
    table: "table_docs"
    column: "content"
    embedding_model: "text-embedding-3-small"
    top_k: 10
```

#### Postgres (`db_orders`, `db_invoice`)
- Usa `query_template` com placeholders (`$1, $2...`) para argumentos.  
- O modelo deve chamar a tool com os parâmetros, ex.:  
  ```
  TOOL:db_invoice 123
  ```
    > Chamar a tool no proprio prompt
- Vai rodar:  
  ```sql
  SELECT status, due_date FROM table_invoices WHERE invoice_id = 123;
  ```

#### Postgres Embedding (`search_docs`)
- Faz busca semântica usando embeddings.  
- O agente pode chamar sem parâmetros:  
  ```
  TOOL:search_docs
  ```
    > Chamar a tool no proprio prompt
- Nesse caso, a query do usuário é usada para gerar embedding e buscar em `table_docs.content`.  
- Exemplo de resultado:  
  ```
  Resultado 1: "Funcionalidade de alertas automáticos para consultas"
  Resultado 2: "Integração de lembretes via WhatsApp"
  ```

---

### Uso no `main.go`

A chamada é a mesma do `Run`/`RouteAndRun`.  
Se o LLM responder algo como:

```
TOOL:db_invoice 123
```

O sistema executará a query no banco e injetará o resultado na resposta final.

Exemplo:

```go
out, err := agent.Run(
    context.Background(),
    cli,
    model,
    embModel,
    sessionID,
    "prompt/instrucoes.md",
    "Quero informações da fatura 123",
    true, // verbose retorna JSON com tool usada
)
```

> As tools são controladas **automaticamente**, não precisa fazer nada no código, apenas chama-las no prompt usando `TOOL:<nome_da_tool>`

---

## Verbose Mode

Se `verbose=true`, a saída é um JSON com metadados:

```json
{
  "tool_requested": "db_invoice",
  "tool_args": ["123"],
  "tool_output": "status=Pago due_date=2024-10-05",
  "final_text": "A fatura 123 está Paga, com vencimento em 05/10/2024."
}
```

---

## Resumo de Cenários

- **Simples:** `agent.Run(...)`
- **Com Cache:** `agent.Run(..., agent.WithCachedContext(...))`
- **Com Router:** `agent.RouteAndRun(...)`
- **Com Tools:** Configurar `tools.yml` e permitir que o LLM invoque com `TOOL:`

---
