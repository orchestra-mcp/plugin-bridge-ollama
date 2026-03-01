package bridgeollama

import (
	"github.com/orchestra-mcp/plugin-bridge-ollama/internal"
	"github.com/orchestra-mcp/sdk-go/plugin"
)

// Register adds all Ollama bridge tools to the builder.
func Register(builder *plugin.PluginBuilder) {
	bp := internal.NewBridgePlugin()
	bp.RegisterTools(builder)
}
