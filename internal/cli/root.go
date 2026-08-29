package cli

import (
	libcli "github.com/shhac/lib-agent-cli/cli"
	_ "github.com/shhac/lib-agent-cli/yaml"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

type globals struct {
	libcli.Globals
	ffmpeg  string
	ffprobe string
}

func newRoot(version string) *cobra.Command {
	g := &globals{}
	root := libcli.NewRoot(libcli.Options{
		Use:           "agent-motion",
		Short:         "Temporal-projection CLI for AI agents",
		Version:       version,
		Globals:       &g.Globals,
		DefaultFormat: output.FormatNDJSON,
		UnknownHint:   "run 'agent-motion usage' for the command overview",
	})
	root.PersistentFlags().StringVar(&g.ffmpeg, "ffmpeg", "ffmpeg", "Path to the FFmpeg executable")
	root.PersistentFlags().StringVar(&g.ffprobe, "ffprobe", "ffprobe", "Path to the FFprobe executable")
	root.AddCommand(projectCommand(g), usageCommand())
	return root
}

func Run(version string) { libcli.Run(newRoot(version)) }
