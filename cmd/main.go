// Command bridge-ollama is the entry point for the bridge.ollama plugin
// binary. It bridges Orchestra MCP to local Ollama models via the Ollama
// REST API. This plugin does NOT require storage -- it manages in-memory
// session state only.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/orchestra-mcp/plugin-bridge-ollama/internal"
	"github.com/orchestra-mcp/sdk-go/plugin"
)

func main() {
	builder := plugin.New("bridge.ollama").
		Version("0.1.0").
		Description("Ollama local LLM bridge plugin").
		Author("Orchestra").
		Binary("bridge-ollama").
		ProvidesAI("ollama")

	bp := internal.NewBridgePlugin()
	bp.RegisterTools(builder)

	p := builder.BuildWithTools()
	p.ParseFlags()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := p.Run(ctx); err != nil {
		log.Fatalf("bridge.ollama: %v", err)
	}
}
