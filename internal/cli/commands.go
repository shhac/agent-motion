package cli

// Commands that answer in text: what this file is, and what happens in it.

import (
	"strings"

	"github.com/shhac/agent-motion/internal/engine"
	output "github.com/shhac/lib-agent-output"
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

func checkCommand(g *globals) *cobra.Command {
	var flags analyseFlags
	var opt engine.CheckOptions
	cmd := &cobra.Command{
		Use:   "check <video>",
		Short: "Assert conditions about a recording and fail if they are not met",
		Long: "Check turns the analysis into a pass or fail, so a visual regression can\n" +
			"break a build instead of waiting to be noticed. Every threshold is opt-in;\n" +
			"with none given it asserts nothing and says so. It exits non-zero on failure,\n" +
			"and every failed assertion names the event that broke it.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.validate(); err != nil {
				return err
			}
			opt.Analyse = flags.options(args[0])
			// Only thresholds the caller actually typed are asserted; an unset
			// zero must not read as "nothing may move at all".
			for _, name := range []string{"max-shift-score", "max-shift-pixels"} {
				if cmd.Flags().Changed(name) {
					opt.Set(name)
				}
			}
			result, err := g.engine().Check(cmd.Context(), opt)
			if err != nil {
				return err
			}
			if err := g.print(cmd, result); err != nil {
				return err
			}
			if !result.Passed {
				return output.New(failureLine(result), output.FixableByHuman).
					WithHint("the full verdict, including which event broke each assertion, is on stdout")
			}
			return nil
		},
	}
	flags.bind(cmd)
	cmd.Flags().Float64Var(&opt.MaxShiftScore, "max-shift-score", 0, "Fail if any layout shift scores above this")
	cmd.Flags().IntVar(&opt.MaxShiftPixels, "max-shift-pixels", 0, "Fail if content moves further than this many pixels")
	cmd.Flags().BoolVar(&opt.NoShift, "no-shift", false, "Fail if any content moves at all")
	cmd.Flags().BoolVar(&opt.NoStall, "no-stall", false, "Fail if continuous activity stops and resumes")
	cmd.Flags().BoolVar(&opt.NoFlicker, "no-flicker", false, "Fail if anything toggles repeatedly")
	cmd.Flags().BoolVar(&opt.Quiet, "quiet", false, "Fail if anything changes at all")
	return cmd
}

// failureLine summarises which assertions failed, for the one-line error.
func failureLine(r *engine.CheckResult) string {
	var failed []string
	for _, a := range r.Assertions {
		if !a.Passed {
			failed = append(failed, a.Name)
		}
	}
	return "check failed: " + strings.Join(failed, ", ")
}

func projectCommand(g *globals) *cobra.Command {
	var flags analyseFlags
	var destination string
	var noLegend bool
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
			result, err := g.engine().WriteProjection(analysis, destination, !noLegend)
			if err != nil {
				return err
			}
			return g.print(cmd, result)
		},
	}
	flags.bind(cmd)
	cmd.Flags().StringVarP(&destination, "output", "o", "", "Destination PNG (default: <video>.temporal.png)")
	cmd.Flags().BoolVar(&noLegend, "no-legend", false, "Leave off the legend band that explains the colours")
	return cmd
}

func activityCommand(g *globals) *cobra.Command {
	var flags analyseFlags
	cmd := &cobra.Command{
		Use:   "activity <video>",
		Short: "List which parts of the frame were busy, and when",
		Long: "Activity is the timeline's spatial counterpart, and the one part of the\n" +
			"activity map that is readable without looking at it. It divides the frame\n" +
			"into a coarse grid and reports every stretch in which a cell was above its\n" +
			"own noise floor, busiest first, one JSON object per line.\n\n" +
			"Use it to narrow down: to find which region to pass to 'sheet --region',\n" +
			"to tell one busy corner from a whole page moving, or to confirm that a\n" +
			"part of the frame you expected to be still was. The cells are coarse on\n" +
			"purpose; the events in 'timeline' carry the exact regions.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.validate(); err != nil {
				return err
			}
			opt := flags.options(args[0])
			opt.Cells = true
			result, err := g.engine().Analyse(cmd.Context(), opt)
			if err != nil {
				return err
			}
			items := make([]any, len(result.Cells))
			for i, c := range result.Cells {
				items[i] = c
			}
			meta := map[string]any{
				"input":                result.Input,
				"grid":                 result.Grid,
				"suitability":          result.Suitability,
				"noise_floor_fraction": result.Params.NoiseFloor,
				"frame_wide":           result.FrameWide,
				"limits": append([]string{
					"A cell is a coarse box, not a shape: a small movement anywhere inside one lights the whole cell.",
					"Only activity local to a cell is listed. An empty list does not mean nothing happened: it means nothing happened in one place while the rest of the frame held still. Read frame_wide for the stretches in which the whole frame moved at once, and 'timeline' for what they were.",
					"A cell is absent when it stayed below its own noise floor, which is not the same as nothing changing there.",
				}, result.Limits...),
			}
			return g.printList(cmd, items, meta)
		},
	}
	flags.bind(cmd)
	return cmd
}
