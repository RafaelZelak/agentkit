# AgentKit

AgentKit is a **Go library** for building modular AI-powered agents with memory, routing, and tool integration.

This README focuses on how to **install**, **configure**, and **use** the library.

---

## Installation

Install AgentKit with:

```bash
go get github.com/RafaelZelak/agentkit@v0.1.0
```

---

## Configuration

AgentKit uses environment variables. You can either export them manually or place them in a `.env` file in your project root.

> Use the .env.example as a reference to configure your environment variables.

---

## Optional: Tools

### Define Tools

Define tools in `tools.yml`. Example for Postgres:

```yaml
- name: db_payment_slip
  description: "Example SQL query to fetch Payment Slip using the ID"
  type: postgres
  conn: "ENV:PGSQL"
  query_template: "SELECT payment_status FROM schema.table WHERE payment_id = $1::int"
```

### Call to Tool

In your `.md` prompt, to call a `TOOL` you just need to describe the moment when the tool should be triggered, and use `TOOL:<tool_name>`. Example:

````md
1. **If the user asks for the status of a payment slip and provides the number (ID)**:
   - You **MUST** reply **only** in the following format:
     ```
     TOOL:db_payment_slip <id>
     ```
````

This way, when the question is about the status of a payment slip, the Agent will run the tool and pass the payment slip id: `TOOL:db_payment_slip <id>`, and using the return, will generate a response.

---

## Example Project Structure (Basic)

```
my-project/
├── .env
├── go.mod
├── go.sum
├── main.go
├── prompt/
│   └── suporte/
│       ├── financeiro.md
│       ├── geral.md
│       ├── router.md
│       ├── suporte.md
│       └── tecnico.md
└── tools.yml (Optional)
```

---

## Example Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/RafaelZelak/agentkit"
)

func main() {
    // Load configuration from env
    cfg, err := agentkit.NewConfigFromEnv()
    if err != nil {
        log.Fatal("Error loading config: ", err)
    }

    // Create new Agent (verbose = true for debug JSON)
    ag, err := agentkit.NewAgent(cfg, true)
    if err != nil {
        log.Fatal("Error creating agent: ", err)
    }

    // Run with router
    out, err := ag.RouteAndRun(
        context.Background(),
        "session123",                // session ID
        "prompt/suporte/suporte.md", // base prompt
        "Hello, I want to know my bill status", // user message
        "prompt/suporte/router.md",  // router
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(out)
}
```

---

## Summary

1. Install the lib with `go get github.com/RafaelZelak/agentkit@v0.1.0`
2. Configure `.env` with your API keys and DB connection
3. Optionally define tools in `tools.yml`
4. Write your own prompts under `prompt/`
5. Run your agent with `go run main.go`

That's it — you have a fully working AI Agent in Go!
