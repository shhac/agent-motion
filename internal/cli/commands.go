package cli

import (
	"github.com/shhac/agent-motion/internal/engine"
	"github.com/spf13/cobra"
)

func inspectCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <video>",
		Short: "Report container and stream facts without decoding pixels",
		Long: "Inspect is the cheap first call: dimensions, frame rate, duration, codec and\n" +
			"whether there is audio. It decodes nothing, so it is safe on a large file.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := g.engine().Inspect(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return g.print(cmd, result)
		},
	}
}

func timelineCommand(g *globals) *cobra.Command {
	var flags analyseFlags
	cmd := &cobra.Command{
		Use:   "timeline <video>",
		Short: "Describe what changes in the video and when",
		Long: "Timeline decodes an interval and returns a narrative, a list of described\n" +
			"events with timestamps and regions, the quiet stretches, and an activity\n" +
			"series. Start here, then narrow the interval or pull frames.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.validate(); err != nil {
				return err
			}
			result, err := g.engine().Analyse(cmd.Context(), flags.options(args[0]))
			if err != nil {
				return err
			}
			return g.print(cmd, result)
		},
	}
	flags.bind(cmd)
	return cmd
}

func projectCommand(g *globals) *cobra.Command {
	var flags analyseFlags
	var destination string
	var legend bool
	cmd := &cobra.Command{
		Use:   "project <video>",
		Short: "Write a spatially aligned activity image, plus the timeline",
		Long: "Project returns everything timeline returns and additionally writes one PNG\n" +
			"in which each pixel keeps its source x,y and its colour encodes how much,\n" +
			"when and how often that pixel changed. It is an activity map, not a frame.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.validate(); err != nil {
				return err
			}
			options := flags.options(args[0])
			options.Native = true
			if destination == "" {
				destination = derived(args[0], ".temporal.png")
			}
			analysis, err := g.engine().Analyse(cmd.Context(), options)
			if err != nil {
				return err
			}
			result, err := g.engine().WriteProjection(analysis, destination, legend)
			if err != nil {
				return err
			}
			return g.print(cmd, result)
		},
	}
	flags.bind(cmd)
	cmd.Flags().StringVarP(&destination, "output", "o", "", "Destination PNG (default: <video>.temporal.png)")
	cmd.Flags().BoolVar(&legend, "legend", true, "Append a legend band below the frame explaining the colours")
	return cmd
}

func framesCommand(g *globals) *cobra.Command {
	var at []string
	var dir string
	var width int
	cmd := &cobra.Command{
		Use:   "frames <video>",
		Short: "Write real source frames at chosen timestamps",
		Long: "Frames extracts unmodified stills so you can see what the picture actually\n" +
			"shows. Use it after timeline has told you which moments matter.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			times, err := parseTimes(at)
			if err != nil {
				return err
			}
			if dir == "" {
				dir = derived(args[0], ".frames")
			}
			result, err := g.engine().Frames(cmd.Context(), engine.FramesOptions{
				Path: args[0], At: times, Dir: dir, Width: width,
			})
			if err != nil {
				return err
			}
			return g.print(cmd, result)
		},
	}
	cmd.Flags().StringSliceVar(&at, "at", nil, "Timestamps in seconds, e.g. --at 3.4,7.1")
	cmd.Flags().StringVar(&dir, "dir", "", "Destination directory (default: <video>.frames)")
	cmd.Flags().IntVar(&width, "width", 0, "Scale frames to this width (default: source width)")
	return cmd
}

func sheetCommand(g *globals) *cobra.Command {
	var flags analyseFlags
	var at []string
	var destination string
	var count, columns, width int
	cmd := &cobra.Command{
		Use:   "sheet <video>",
		Short: "Write one labelled grid of frames covering the video",
		Long: "Sheet packs many real frames into a single image, each captioned with its\n" +
			"timestamp. With no --at it analyses the video first and shows the moments\n" +
			"that matter, so one image and one call describe the whole recording.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.validate(); err != nil {
				return err
			}
			times, err := parseTimes(at)
			if err != nil {
				return err
			}
			if destination == "" {
				destination = derived(args[0], ".sheet.png")
			}
			result, err := g.engine().Sheet(cmd.Context(), engine.SheetOptions{
				Path: args[0], At: times, Count: count, Columns: columns,
				Width: width, Output: destination, Analyse: flags.options(args[0]),
			})
			if err != nil {
				return err
			}
			return g.print(cmd, result)
		},
	}
	flags.bind(cmd)
	cmd.Flags().StringSliceVar(&at, "at", nil, "Timestamps in seconds; omit to let the analysis choose")
	cmd.Flags().IntVar(&count, "count", 12, "How many frames to show when the analysis chooses them")
	cmd.Flags().IntVar(&columns, "columns", 0, "Grid columns (default: chosen to stay roughly square)")
	cmd.Flags().IntVar(&width, "width", 320, "Thumbnail width in pixels")
	cmd.Flags().StringVarP(&destination, "output", "o", "", "Destination PNG (default: <video>.sheet.png)")
	return cmd
}
