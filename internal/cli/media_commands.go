package cli

// Commands that produce or compare pictures.

import (
	"github.com/shhac/agent-motion/internal/engine"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

func framesCommand(g *globals) *cobra.Command {
	var at []string
	var dir, region, during string
	var width, pad, count int
	cmd := &cobra.Command{
		Use:   "frames <video>",
		Short: "Write real source frames at chosen timestamps",
		Long: "Frames extracts unmodified stills so you can see what the picture actually\n" +
			"shows. Use it after timeline has told you which moments matter.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			times, err := timestamps(at, during, count)
			if err != nil {
				return err
			}
			crop, err := parseRegion(region, pad)
			if err != nil {
				return err
			}
			if dir == "" {
				dir = derived(args[0], ".frames")
			}
			result, err := g.engine().Frames(cmd.Context(), engine.FramesOptions{
				Path: args[0], At: times, Dir: dir, Width: width, Region: crop,
			})
			if err != nil {
				return err
			}
			return g.print(cmd, result)
		},
	}
	cmd.Flags().StringSliceVar(&at, "at", nil, "Timestamps in seconds, e.g. --at 3.4,7.1")
	cmd.Flags().StringVar(&during, "during", "", "Sample evenly across a window, e.g. --during 13.07:13.40; paste an event's span")
	cmd.Flags().IntVar(&count, "count", 6, "How many frames --during takes")
	cmd.Flags().StringVar(&dir, "dir", "", "Destination directory (default: <video>.frames)")
	cmd.Flags().IntVar(&width, "width", 0, "Scale frames to this width (default: source width)")
	cmd.Flags().StringVar(&region, "region", "", "Crop to x0,y0,x1,y1 in source pixels; paste an event's region_xyxy")
	cmd.Flags().IntVar(&pad, "pad", 0, "Widen --region by this many pixels on every side")
	return cmd
}

func compareCommand(g *globals) *cobra.Command {
	var at []string
	var region, destination string
	var pad int
	var threshold float64
	cmd := &cobra.Command{
		Use:   "compare <video>",
		Short: "Say exactly how two moments of the video differ",
		Long: "Compare answers the question every other command sidesteps: is this the\n" +
			"same as it was. It measures two arbitrary timestamps against each other and\n" +
			"reports an exact pixel count, so \"it came back\" and \"it only looks similar\"\n" +
			"stop being a matter of eyeballing two stills. With --output it also draws\n" +
			"the difference, which is the only way to see a change of a pixel or two.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if threshold < 0 || threshold > 255 {
				return output.New("--threshold must be in the range 0..255", output.FixableByAgent)
			}
			times, err := parseTimes(at)
			if err != nil {
				return err
			}
			crop, err := parseRegion(region, pad)
			if err != nil {
				return err
			}
			result, err := g.engine().Compare(cmd.Context(), engine.CompareOptions{
				Path: args[0], At: times, Threshold: threshold, Region: crop, Output: destination,
			})
			if err != nil {
				return err
			}
			return g.print(cmd, result)
		},
	}
	cmd.Flags().StringSliceVar(&at, "at", nil, "The two timestamps to compare, e.g. --at 14.9,18.5")
	cmd.Flags().Float64Var(&threshold, "threshold", engine.DefaultThreshold, "Ignore per-pixel differences at or below this 0..255 value")
	cmd.Flags().StringVar(&region, "region", "", "Compare only x0,y0,x1,y1 in source pixels; paste an event's region_xyxy")
	cmd.Flags().IntVar(&pad, "pad", 0, "Widen --region by this many pixels on every side")
	cmd.Flags().StringVarP(&destination, "output", "o", "", "Draw the difference to this PNG")
	return cmd
}

func sheetCommand(g *globals) *cobra.Command {
	var flags analyseFlags
	var at []string
	var destination, region, during string
	var count, columns, width, pad int
	var quick bool
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
			times, err := timestamps(at, during, count)
			if err != nil {
				return err
			}
			crop, err := parseRegion(region, pad)
			if err != nil {
				return err
			}
			if destination == "" {
				destination = derived(args[0], ".sheet.png")
			}
			result, err := g.engine().Sheet(cmd.Context(), engine.SheetOptions{
				Path: args[0], At: times, Count: count, Columns: columns,
				Width: width, Output: destination, Region: crop, Quick: quick,
				Analyse: flags.options(args[0]),
			})
			if err != nil {
				return err
			}
			return g.print(cmd, result)
		},
	}
	flags.bind(cmd)
	cmd.Flags().StringSliceVar(&at, "at", nil, "Timestamps in seconds; omit to let the analysis choose")
	cmd.Flags().StringVar(&during, "during", "", "Sample evenly across a window, e.g. --during 13.07:13.40; paste an event's span")
	cmd.Flags().IntVar(&count, "count", 12, "How many frames to show, for --during or when the analysis chooses")
	cmd.Flags().IntVar(&columns, "columns", 0, "Grid columns (default: chosen to stay roughly square)")
	cmd.Flags().IntVar(&width, "width", 320, "Thumbnail width in pixels")
	cmd.Flags().StringVarP(&destination, "output", "o", "", "Destination PNG (default: <video>.sheet.png)")
	cmd.Flags().StringVar(&region, "region", "", "Crop every tile to x0,y0,x1,y1 in source pixels; paste an event's region_xyxy")
	cmd.Flags().IntVar(&pad, "pad", 0, "Widen --region by this many pixels on every side")
	cmd.Flags().BoolVar(&quick, "quick", false, "Skip the analysis pass; faster, but tiles lose their event labels")
	return cmd
}
