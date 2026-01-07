package main

import (
	"fmt"
	"log"

	"github.com/RafaelZelak/agentkit"
	"github.com/joho/godotenv"

	_ "github.com/RafaelZelak/agentkit/scripts"
)

func main() {
	// Load .env file
	_ = godotenv.Load()

	// Load agents from YAML
	manager, err := agentkit.LoadAgents("agents.yml")
	if err != nil {
		log.Fatal("Failed to load agents: ", err)
	}
	defer manager.Close()

	// List loaded agents
	fmt.Println("Loaded agents:", manager.List())

	// Test chat with agent "suporte"
	sessionID := "session123"
	message := "Hello, my name is Rafael"

	out, err := manager.Chat("suporte", sessionID, message)
	if err != nil {
		log.Fatal("Chat error: ", err)
	}

	fmt.Println(out)
}
