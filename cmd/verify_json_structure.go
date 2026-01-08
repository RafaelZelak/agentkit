package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/RafaelZelak/agentkit/internal/agent"
	"github.com/RafaelZelak/agentkit/internal/memory"
	"github.com/RafaelZelak/agentkit/internal/openai"
	"github.com/RafaelZelak/agentkit/internal/tools"
)

func main() {
	// 1. Setup Mock OpenAI Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Basic mock response
		w.Header().Set("Content-Type", "application/json")
		
		// Check if it's an embedding request
		if strings.Contains(r.URL.Path, "embeddings") {
			w.Write([]byte(`{
				"data": [{"embedding": [0.1, 0.2, 0.3]}]
			}`))
			return
		}

		// Chat completion request
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		
		messages := req["messages"].([]any)
		lastMsg := messages[len(messages)-1].(map[string]any)
		content := lastMsg["content"].([]any)[0].(map[string]any)["text"].(string)

		var outputText string
		if strings.Contains(content, "Regras de roteamento") {
			// Router response
			outputText = "financeiro.md"
		} else if strings.Contains(content, "TOOL:") {
			// Tool execution response (simulated)
			outputText = "Final answer after tool"
		} else {
			// Standard response, maybe simulating a tool call
			// We want to simulate a tool call first
			// But wait, the agent logic is:
			// 1. Router -> "financeiro.md"
			// 2. Run -> LLM -> "TOOL: search_boletos 123"
			// 3. Run -> Tool Exec -> output
			// 4. Run -> LLM -> "Final answer"
			
			// We need to be stateful or just return a fixed sequence based on content?
			// The agent sends the history.
			
			// Let's just return a simple response first to check structure.
			// If we want to test tools, we need to return "TOOL: ..."
			
			// For simplicity, let's just return a final text first to see if the structure wraps it.
			outputText = "Olá, sou o assistente financeiro."
		}

		resp := map[string]any{
			"id": "chatcmpl-123",
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": outputText,
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens": 10,
				"completion_tokens": 5,
				"total_tokens": 15,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 2. Setup Env Vars
	os.Setenv("OPENAI_BASE_URL", server.URL)
	os.Setenv("OPENAI_CHAT_PATH", "/v1/chat/completions")
	os.Setenv("OPENAI_EMBEDDINGS_PATH", "/v1/embeddings")

	// 3. Setup Filesystem
	tmpDir, err := os.MkdirTemp("", "agent_test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	routerPath := filepath.Join(tmpDir, "router.md")
	os.WriteFile(routerPath, []byte("Router Prompt"), 0644)
	
	os.WriteFile(filepath.Join(tmpDir, "financeiro.md"), []byte("Financeiro Prompt"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "geral.md"), []byte("Geral Prompt"), 0644)

	// 4. Initialize Components
	cli := openai.NewClient("fake-key")

	// Setup Mock DB
	db, mock, err := sqlmock.New()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Expect RetrieveRecent
	mock.ExpectQuery("SELECT role, text FROM .*chat_memory").
		WithArgs("session123", 4).
		WillReturnRows(sqlmock.NewRows([]string{"role", "text"}))

	// Expect LoadBoletoStatus
	mock.ExpectQuery("SELECT m.value FROM .*metadata m").
		WithArgs("session123").
		WillReturnRows(sqlmock.NewRows([]string{"value"}))

	// Expect SaveEmbeddedMessage (user)
	mock.ExpectQuery("INSERT INTO .*chat_memory").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	// Expect SaveEmbeddedMessage (assistant)
	mock.ExpectQuery("INSERT INTO .*chat_memory").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
	
	// Expect SaveMetadata (response_raw) - optional depending on implementation
	// The implementation calls SaveMetadata if resp.Raw != nil.
	// Our mock client returns Raw map, so it will call SaveMetadata.
	mock.ExpectExec("INSERT INTO .*metadata").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mem := memory.NewStoreWithDB(db, memory.Config{Schema: "public", EmbeddingDim: 1536})

	// 5. Run RouteAndRun
	ctx := context.Background()
	output, _, err := agent.RouteAndRun(
		ctx,
		cli,
		"gpt-4o",
		"text-embedding-3-small",
		"session123",
		"base.md",
		"Olá, tenho um boleto",
		routerPath,
		true, // verbose=true to get JSON
		mem,
		&tools.Registry{}, // empty registry for now
	)

	if err != nil {
		panic(err)
	}

	// 6. Verify Output
	fmt.Println("Output JSON:")
	fmt.Println(output)

	var resp map[string]any
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		panic("Output is not valid JSON: " + err.Error())
	}

	// Check structure
	if _, ok := resp["prompt"]; !ok {
		panic("Missing 'prompt' block")
	}
	if _, ok := resp["final_text"]; !ok {
		panic("Missing 'final_text' block")
	}
	
	fmt.Println("Verification Successful! Structure is correct.")
}
