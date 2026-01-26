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
├── tools/              # Diretório de configuração de ferramentas
│   └── suporte.yml
└── funcs/              # (Opcional) Diretório de funções Go para templates
    └── time.go
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

    # Ferramentas (Opcional)
    tools_path: tools/suporte.yml # Arquivo de definição das tools

    # Functions (Opcional)
    functions_path: funcs/ # Diretório onde as funções Go estão (para validação)

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
    
    // Importe suas funções customizadas para que o init() deles seja executado
    // Isso é necessário para registrar as funções no sistema
    _ "my-project/funcs"
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

## Funções em Prompts (Functions)

O AgentKit permite que você execute funções Go diretamente dentro dos seus arquivos de prompt usando sintaxe de template. Diferente das **Tools** (que são executadas pela LLM durante a conversa), as **Functions** são executadas **antes** do prompt ser enviado para a OpenAI, substituindo os templates pelos resultados em tempo real.

**Diferença chave:**
- **Tools**: Executadas pela LLM durante a conversa (`TOOL:nome_tool args`)
- **Functions**: Executadas automaticamente ao processar o prompt (`{{ function.name }}`)

### Quando Usar Functions?

Functions são ideais para:
- **Informações dinâmicas**: Horário atual, data, ambiente (dev/prod)
- **Transformações simples**: Formatação de texto, cálculos rápidos
- **Valores contextuais**: Nome do usuário, configurações do sistema
- **Personalização**: Saudações baseadas em hora, mensagens condicionais

### 1. Configuração no `agents.yml` (Opcional)

Assim como `tools_path`, o `functions_path` é **opcional**. Ele serve apenas para validação - o sistema verifica se o diretório existe ao criar o agente.

```yaml
agents:
  - name: suporte
    # ... outras configurações ...
    
    # Functions (Opcional)
    functions_path: funcs/ # Diretório onde suas funções Go estão
```

**Importante:** Mesmo que você não defina `functions_path`, as funções ainda funcionarão se você importar o pacote no `main.go`. O `functions_path` é apenas uma validação de que o diretório existe.

### 2. Criar Funções Go

Crie um pacote (ex: `funcs/time.go`) com suas funções. Cada função deve retornar `string` ou `(string, error)`.

```go
package funcs

import (
    "fmt"
    "time"
    
    "github.com/RafaelZelak/agentkit/sdk"
)

// init() é executado automaticamente quando o pacote é importado
func init() {
    // Registra a função "time.now" que pode ser chamada como {{ time.now }}
    sdk.RegisterGoFunction("time.now", Now)
    
    // Registra a função "time.greeting" que aceita um argumento
    sdk.RegisterGoFunction("time.greeting", Greeting)
}

// Now retorna uma saudação baseada no horário atual
func Now() string {
    hour := time.Now().Hour()
    
    if hour >= 6 && hour < 12 {
        return "Bom Dia"
    } else if hour >= 12 && hour < 18 {
        return "Boa Tarde"
    } else {
        return "Boa Noite"
    }
}

// Greeting retorna uma saudação personalizada
// Parâmetros string podem ser opcionais (valor padrão: string vazia)
func Greeting(name string) string {
    greeting := Now()
    if name == "" {
        name = "usuário"
    }
    return fmt.Sprintf("%s, %s", greeting, name)
}
```

### 3. Registrar Funções com `RegisterGoFunction`

O `sdk.RegisterGoFunction` aceita dois parâmetros:

1. **Nome da função** (string): O nome que você usará no template, no formato `"pacote.funcao"` (ex: `"time.now"`, `"math.add"`)
2. **Função Go**: A referência da função (sem parênteses)

**Assinaturas suportadas:**
- `func() string` - Sem argumentos
- `func() (string, error)` - Sem argumentos, com tratamento de erro
- `func(arg1 string) string` - Com argumentos
- `func(arg1 string, arg2 int) string` - Múltiplos argumentos
- `func(args ...interface{}) (string, error)` - Assinatura genérica

**Argumentos opcionais:**
Parâmetros do tipo `string` podem ser omitidos na chamada. Se não fornecidos, receberão o valor padrão (string vazia `""`).

### 4. Importar no `main.go`

**Crucial:** Você **deve** importar o pacote de funções no `main.go` para que o `init()` seja executado e as funções sejam registradas.

```go
package main

import (
    "fmt"
    "log"
    "github.com/RafaelZelak/agentkit"
    "github.com/joho/godotenv"

    // Importe suas funções - o init() será executado automaticamente
    _ "github.com/RafaelZelak/agentkit/funcs"
    
    // Importe seus scripts também (se houver)
    _ "github.com/RafaelZelak/agentkit/scripts"
)

func main() {
    _ = godotenv.Load()
    
    manager, err := agentkit.LoadAgents("agents.yml")
    if err != nil {
        log.Fatal("Erro ao carregar agentes:", err)
    }
    defer manager.Close()
    
    // ... resto do código
}
```

**Por que o `_` antes do import?**
O `_` indica um import "em branco" - você está importando o pacote apenas para executar o `init()`, não para usar diretamente no código.

### 5. Usar nos Prompts

Use a sintaxe `{{ function.name }}` ou `{{ function.name(args) }}` diretamente nos seus arquivos `.md`:

**Exemplo (`prompts/suporte/suporte.md`):**

```markdown
Você é **Bia**, assistente virtual do Suporte.

Quando o usuário cumprimentar, responda com {{ time.now }}

Para personalizar: Olá {{ time.greeting("João") }}, como posso ajudar?
```

**Resultado processado antes de enviar para a OpenAI:**

```markdown
Você é **Bia**, assistente virtual do Suporte.

Quando o usuário cumprimentar, responda com Bom Dia

Para personalizar: Olá Bom Dia, João, como posso ajudar?
```

### 6. Sintaxe de Templates

**Sem argumentos:**
```markdown
{{ time.now }}
```

**Com argumentos:**
```markdown
{{ time.greeting("usuário") }}
{{ math.add(1, 2) }}
{{ format.currency(100.50) }}
```

**Argumentos opcionais:**
```markdown
{{ time.greeting }}  # name receberá "" (string vazia)
{{ time.greeting("João") }}  # name receberá "João"
```

**Tipos de argumentos suportados:**
- Strings: `{{ func("texto") }}`
- Inteiros: `{{ func(123) }}`
- Floats: `{{ func(12.34) }}`

### 7. Execução Dinâmica

**Importante:** As funções são executadas **a cada vez** que o prompt é processado. Isso significa que:

- `{{ time.now }}` retornará valores diferentes dependendo do horário
- Funções que dependem de estado externo sempre terão dados atualizados
- Não há cache - cada chamada é uma execução nova

### 8. Verbose Mode e Functions

Quando `verbose: true`, o JSON de resposta inclui um array `functions` mostrando todas as funções executadas:

```json
{
  "final_text": "Bom Dia! Como posso ajudar?",
  "usage": { ... },
  "functions": [
    {
      "template": "{{ time.now }}",
      "result": "Bom Dia"
    },
    {
      "template": "{{ time.greeting(\"usuário\") }}",
      "result": "Bom Dia, usuário"
    }
  ]
}
```

Isso é útil para debug e para entender quais funções foram executadas e seus resultados.

### 9. Exemplo Completo

**Estrutura:**
```
my-project/
├── funcs/
│   └── time.go
├── prompts/
│   └── suporte/
│       └── suporte.md
├── agents.yml
└── main.go
```

**`funcs/time.go`:**
```go
package funcs

import (
    "fmt"
    "time"
    "github.com/RafaelZelak/agentkit/sdk"
)

func init() {
    sdk.RegisterGoFunction("time.now", Now)
    sdk.RegisterGoFunction("time.greeting", Greeting)
}

func Now() string {
    hour := time.Now().Hour()
    if hour >= 6 && hour < 12 {
        return "Bom Dia"
    } else if hour >= 12 && hour < 18 {
        return "Boa Tarde"
    } else {
        return "Boa Noite"
    }
}

func Greeting(name string) string {
    greeting := Now()
    if name == "" {
        name = "usuário"
    }
    return fmt.Sprintf("%s, %s", greeting, name)
}
```

**`prompts/suporte/suporte.md`:**
```markdown
Você é **Bia**, assistente virtual.

Sempre comece suas respostas com {{ time.now }}.
```

**`agents.yml`:**
```yaml
agents:
  - name: suporte
    # ... outras configs ...
    functions_path: funcs/
```

**`main.go`:**
```go
import (
    _ "github.com/RafaelZelak/agentkit/funcs"
)
```

**Resultado:** A cada mensagem, o prompt será processado e `{{ time.now }}` será substituído pela saudação apropriada antes de ser enviado para a OpenAI.

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
  "functions": [
    {
      "template": "{{ time.now }}",
      "result": "Bom Dia"
    }
  ],
  "usage": {
    "prompt_tokens": 500,
    "completion_tokens": 50,
    "total_tokens": 550
  },
  "final_text": "O valor atualizado com juros é R$ 1.200,00."
}
```

Se `verbose: false`, o retorno será apenas a string: `"O valor atualizado com juros é R$ 1.200,00."`

---

## Prompts e Tools (Guia de Engenharia de Prompt)

Para garantir que a LLM execute as tools corretamente, especialmente quando há informações em memória, é crucial ser explícito em seus prompts.

### Regras de Ouro:

1.  **Forçe a Execução**: Use imperativos como "Você DEVE executar a tool" ou "É OBRIGATÓRIO consultar".
2.  **Desconfie da Memória**: Instrua o modelo a não confiar cegamente em dados antigos se a pergunta for sobre o estado _atual_.
3.  **Condicionais Claras**: Defina exatamente _quando_ a tool deve ser chamada.

### Exemplo (Ruim vs Bom):

**Ruim (Ambíguo):**

> "Se o usuário perguntar de boletos, veja se tem algum."
> _(A LLM pode achar que "ver se tem algum" significa olhar no histórico do chat)_

**Bom (Explícito):**

> "Se o usuário perguntar sobre boletos, você **DEVE** executar a tool `search_boletos`. Não responda com base no histórico, busque sempre o dado atualizado."

### Snippet Recomendado para seus Prompts:

Adicione isso ao final das instruções de suas tools no arquivo `.md`:

```markdown
> IMPORTANTE: Mesmo que você tenha informações sobre esse assunto na memória ou contexto anterior, se o usuário perguntar o status atual ou solicitar uma nova verificação, você **DEVE** executar a tool novamente para buscar a informação mais recente. Não assuma que o estado anterior ainda é válido.
```
