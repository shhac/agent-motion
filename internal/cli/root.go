package cli

import (
	"github.com/shhac/agent-motion/internal/engine"
	"github.com/shhac/agent-motion/internal/video"
	libcli "github.com/shhac/lib-agent-cli/cli"
	_ "github.com/shhac/lib-agent-cli/yaml"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

type globals struct {
	libcli.Globals
	ffmpeg  string
	ffprobe string

	// decoder overrides the FFmpeg decoder in tests.
	decoder video.Decoder
}

// engine builds the command engine, defaulting to locally installed FFmpeg.
func (g *globals) engine() *engine.Engine {
	if g.decoder != nil {
		return engine.New(g.decoder)
	}
	return engine.New(video.NewFFmpeg(g.ffmpeg, g.ffprobe))
}

// print writes one resource in the resolved format.
func (g *globals) print(cmd *cobra.Command, value any) error {
	format, err := output.ResolveFormat(g.Format, output.FormatJSON)
	if err != nil {
		return err
	}
	return output.Print(cmd.OutOrStdout(), value, format, output.PruneEmpty)
}

// newRoot builds the command tree. A non-nil decoder replaces locally
// installed FFmpeg, which is how the command tests run without a decoder.
func newRoot(version string, dec video.Decoder) *cobra.Command {
	g := &globals{decoder: dec}
	root := libcli.NewRoot(libcli.Options{
		Use:           "agent-motion",
		Short:         "Temporal video analysis CLI for AI agents",
		Version:       version,
		Globals:       &g.Globals,
		DefaultFormat: output.FormatJSON,
		UnknownHint:   "run 'agent-motion usage' for the command overview",
	})
	root.PersistentFlags().StringVar(&g.ffmpeg, "ffmpeg", "ffmpeg", "Path to the FFmpeg executable")
	root.PersistentFlags().StringVar(&g.ffprobe, "ffprobe", "ffprobe", "Path to the FFprobe executable")
	root.AddCommand(
		inspectCommand(g),
		timelineCommand(g),
		projectCommand(g),
		framesCommand(g),
		sheetCommand(g),
		usageCommand(),
	)
	return root
}

// Run executes the CLI.
func Run(version string) { libcli.Run(newRoot(version, nil)) }
