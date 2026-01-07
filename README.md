# AgentKit

**AgentKit** é uma biblioteca Go modular e poderosa para construir agentes de Inteligência Artificial com memória persistente, roteamento inteligente de prompts e integração extensível de ferramentas.

Projetada para ser simples de usar, mas robusta o suficiente para aplicações complexas multi-agente.

---

## Instalação

Adicione o AgentKit ao seu projeto Go:

```bash
go get github.com/RafaelZelak/agentkit@latest
```

---

## Quick Start

A maneira mais fácil de começar é utilizando o arquivo de configuração `agents.yml`.

### 1. Estrutura do Projeto Recomendada

Organize seu projeto para manter prompts e configurações limpos:

```
my-project/
├── agents.yml          # Configuração central dos agentes
├── .env                # Variáveis de ambiente (API Keys, DB)
├── main.go             # Ponto de entrada da aplicação
├── prompts/            # Diretório de prompts
│   ├── suporte/
│   │   ├── base.md
│   │   └── router.md
│   └── vendas/
│       └── base.md
└── tools/              # Diretório de configuração de ferramentas
    └── suporte.yml
```

### 2. Configuração (`agents.yml`)

O arquivo `agents.yml` permite definir múltiplos agentes, cada um com sua própria memória, modelo e ferramentas.

```yaml
agents:
  - name: suporte
    description: "Agente de Suporte Técnico Nível 1"

    # Configuração da IA
    api_key: ${OPENAI_API_KEY} # Suporta expansão de variáveis de ambiente
    gpt_model: gpt-4o # Modelo de chat
    embedding_model: text-embedding-3-small # Modelo de embedding (opcional)
    embedding_dim: 1536 # Dimensão do embedding (padrão: 1536)

    # Memória Persistente (PostgreSQL)
    dsn: ${PGSQL_DSN} # String de conexão Postgres
    schema: suporte_memory # Schema isolado para este agente

    # Prompts
    prompts_dir: prompts/suporte/ # Diretório base dos prompts
    base_prompt: prompts/suporte/base.md # Prompt principal
    router_prompt: prompts/suporte/router.md # (Opcional) Ativa o Roteamento Inteligente

    # Ferramentas
    tools_path: tools/suporte.yml # Arquivo de definição das tools

    # Configurações Extras
    timeout: 60s # Timeout global para respostas
    verbose: true # Retorna JSON detalhado com metadados e uso de tokens
```

### 3. Exemplo de Uso (`main.go`)

```go
package main

import (
    "fmt"
    "log"
    "github.com/RafaelZelak/agentkit"

    // Importe seus scripts customizados para que o init() deles seja executado
    _ "my-project/scripts"
)

func main() {
    // 1. Carrega todos os agentes definidos no YAML
    manager, err := agentkit.LoadAgents("agents.yml")
    if err != nil {
        log.Fatal("Erro ao carregar agentes:", err)
    }
    defer manager.Close() // Fecha conexões de banco de dados

    // 2. Interage com um agente específico pelo nome
    sessionID := "user-session-123" // ID da sessão para manter o contexto da conversa

    response, err := manager.Chat("suporte", sessionID, "Tenho uma dúvida sobre minha fatura")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Resposta do Agente:")
    fmt.Println(response)
}
```

---

## Ferramentas (Tools)

O AgentKit permite que seus agentes executem ações reais através de **Tools**. As tools são definidas em arquivos YAML (ex: `tools/suporte.yml`) e podem ser de três tipos:

### 1. `postgres` (Banco de Dados)

Executa consultas SQL seguras diretamente no banco de dados. Ideal para buscar informações transacionais (status de pedidos, saldo, cadastro).

**Configuração (`tools/suporte.yml`):**

```yaml
- name: consultar_fatura
  description: "Busca o status e valor de uma fatura pelo ID"
  type: postgres
  conn: "ENV:PGSQL_DSN" # Lê a string de conexão da variável de ambiente
  query_template: "SELECT status, valor, vencimento FROM faturas WHERE id = $1::int"
```

**Como o Agente usa:**
O agente gera internamente o comando: `TOOL:consultar_fatura 12345`

---

### 2. `postgres_embedding` (Busca Semântica / RAG)

Realiza busca vetorial em uma tabela PostgreSQL com suporte a `pgvector`. Essencial para criar sistemas de RAG (Retrieval-Augmented Generation), permitindo que o agente consulte grandes bases de conhecimento.

**Pré-requisitos:**

- Extensão `vector` habilitada no Postgres.
- Tabela com uma coluna do tipo `vector`.

**Configuração (`tools/suporte.yml`):**

```yaml
- name: buscar_documentacao
  description: "Busca na base de conhecimento técnica por similaridade"
  type: postgres_embedding
  conn: "ENV:PGSQL_DSN"
  table: "documentacao" # Tabela alvo
  column: "conteudo" # Coluna de texto para retorno
  embedding_model: "text-embedding-3-small" # Modelo usado para gerar o vetor da query
  top_k: 5 # Número de resultados mais relevantes a retornar
```

**Como o Agente usa:**
O agente gera: `TOOL:buscar_documentacao "como configurar roteador modelo X"`

---

### 3. `script` (Funções Go Customizadas)

Permite executar lógica de negócio complexa escrita em Go. Você registra uma função Go e a expõe para o agente.

**Passo 1: Definir no YAML (`tools/suporte.yml`)**

```yaml
- name: calcular_juros
  description: "Calcula juros compostos de um boleto atrasado"
  type: script
  function: "CalcJuros($1, $2)" # $1 será o valor, $2 será os meses de atraso
```

**Passo 2: Registrar no Go**
Crie um pacote (ex: `scripts/financeiro.go`) e registre a função no `init()`.

```go
package scripts

import (
    "fmt"
    "strconv"
    "github.com/RafaelZelak/agentkit/sdk"
)

func init() {
    // Registra a função "CalcJuros" para ser usada pelo AgentKit
    sdk.RegisterScript("CalcJuros", CalcularJuros)
}

// A assinatura deve ser sempre: func(args ...string) (string, error)
func CalcularJuros(args ...string) (string, error) {
    if len(args) < 2 {
        return "", fmt.Errorf("argumentos insuficientes")
    }

    valor, _ := strconv.ParseFloat(args[0], 64)
    meses, _ := strconv.Atoi(args[1])

    total := valor * (1 + 0.02*float64(meses)) // Exemplo simples

    return fmt.Sprintf("%.2f", total), nil
}
```

---

## Roteamento Inteligente (Router)

O AgentKit possui um sistema nativo de roteamento de intenções. Isso permite que um único agente "mude de personalidade" ou utilize prompts especializados dependendo do que o usuário pede.

**Como funciona:**

1. Configure `router_prompt` no `agents.yml`.
2. Crie arquivos `.md` na pasta `prompts_dir` (ex: `financeiro.md`, `tecnico.md`, `vendas.md`).
3. O `router_prompt` deve instruir o modelo a classificar a intenção e retornar apenas o nome do arquivo (ex: `financeiro.md`).

**Fluxo:**
`Usuário` -> `Router (Analisa)` -> `Seleciona Prompt Especialista` -> `Agente Executa (com Prompt Especialista)`

Isso melhora drasticamente a qualidade das respostas em domínios complexos.

---

## Verbose Mode & JSON Output

Para integrações via API, é útil receber não apenas o texto da resposta, mas também metadados sobre o processo de decisão e uso de recursos.

Ao definir `verbose: true` no `agents.yml`, o método `Chat` retornará uma string JSON contendo:

```json
{
  "router_enabled": true,
  "base_prompt": "prompts/suporte/base.md",
  "user_message": "Qual o valor dos juros para 10 meses?",
  "session_id": "user-123",
  "chosen_prompt": "financeiro.md",
  "tool_requested": "calcular_juros",
  "tool_args": ["1000", "10"],
  "tool_output": "1200.00",
  "usage": {
    "prompt_tokens": 500,
    "completion_tokens": 50,
    "total_tokens": 550
  },
  "final_text": "O valor atualizado com juros é R$ 1.200,00."
}
```

Se `verbose: false`, o retorno será apenas a string: `"O valor atualizado com juros é R$ 1.200,00."`
