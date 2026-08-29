package cli

import (
	"path/filepath"
	"strings"

	"github.com/shhac/agent-motion/internal/projection"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

func projectCommand(g *globals) *cobra.Command {
	var start, end float64
	var threshold int
	var destination string

	cmd := &cobra.Command{
		Use:   "project <video>",
		Short: "Write a spatially aligned temporal projection PNG and return its metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if start < 0 {
				return output.New("--start must be zero or greater", output.FixableByAgent)
			}
			if end != 0 && end <= start {
				return output.New("--end must be greater than --start", output.FixableByAgent)
			}
			if threshold < 0 || threshold > 255 {
				return output.New("--threshold must be in the range 0..255", output.FixableByAgent)
			}
			if destination == "" {
				destination = defaultOutput(args[0])
			}
			result, err := projection.Project(cmd.Context(), projection.Config{
				Input: args[0], Output: destination, Start: start, End: end,
				Threshold: uint8(threshold), FFmpeg: g.ffmpeg, FFprobe: g.ffprobe,
			})
			if err != nil {
				return err
			}
			format, err := output.ResolveFormat(g.Format, output.FormatJSON)
			if err != nil {
				return err
			}
			return output.Print(cmd.OutOrStdout(), result, format, output.PruneEmpty)
		},
	}
	cmd.Flags().Float64Var(&start, "start", 0, "Interval start in seconds")
	cmd.Flags().Float64Var(&end, "end", 0, "Interval end in seconds (default: end of video)")
	cmd.Flags().IntVar(&threshold, "threshold", 12, "Suppress mean RGB differences at or below this value (0..255)")
	cmd.Flags().StringVarP(&destination, "output", "o", "", "Destination PNG (default: <video>.temporal.png)")
	return cmd
}

func defaultOutput(input string) string {
	ext := filepath.Ext(input)
	return strings.TrimSuffix(input, ext) + ".temporal.png"
}
