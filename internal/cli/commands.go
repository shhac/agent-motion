package cli

// Commands that answer in text: what this file is, and what happens in it.

import "github.com/spf13/cobra"

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
