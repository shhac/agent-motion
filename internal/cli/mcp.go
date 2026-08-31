package cli

import (
	agentmcp "github.com/shhac/lib-agent-mcp"
	"github.com/spf13/cobra"
)

// registerMCP adds `agent-motion mcp` — an MCP stdio server reflected from the
// cobra tree, so any MCP client gets the same surface as the CLI. It must be
// called last, once every command is registered, or the generated tool surface
// is missing whatever was added after it.
//
// The commands that only answer questions are marked read-only. The rest are
// exposed without that hint because they write an image or a directory of
// frames. None of them ever modifies the source video.
func registerMCP(root *cobra.Command) {
	readOnly := map[string]bool{
		"inspect": true, "timeline": true, "activity": true, "check": true, "usage": true,
	}
	for _, cmd := range root.Commands() {
		switch cmd.Name() {
		case "inspect", "timeline", "activity", "check", "sheet", "project", "frames", "compare", "usage":
			agentmcp.Expose(cmd)
			if readOnly[cmd.Name()] {
				agentmcp.ReadOnly(cmd)
			}
		}
	}
	root.AddCommand(agentmcp.Command(root, agentmcp.WithHiddenFlags("color")))
}
