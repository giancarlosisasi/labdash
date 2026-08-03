package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/giancarlosisasi/labdash/internal/tui/preview"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// defaultWidth is what the sheet is drawn at when the width cannot be read —
// stdout is a pipe, or the terminal does not answer. It is the width every
// screen in research/16-screens-and-flows.md is drawn at.
const defaultWidth = 120

func newThemeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "theme",
		Short: "Inspect labdash's appearance",
	}

	cmd.AddCommand(newThemePreviewCmd())

	return cmd
}

func newThemePreviewCmd() *cobra.Command {
	var (
		name    string
		icons   string
		noColor bool
		width   int
		colors  []string
	)

	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Render every colour token and status glyph in this terminal",
		Long: "Render labdash's whole visual vocabulary in the terminal you are sitting at.\n\n" +
			"Every semantic token, every pipeline status, every merge blocker and every\n" +
			"marker, each as the triple it is drawn as: a glyph, a colour and a word. Cover\n" +
			"the colours with your hand and every state is still readable — that is the\n" +
			"property the whole design system rests on.\n\n" +
			"It contacts no network and reads no credential.\n\n" +
			"  labdash theme preview --theme=ember-light\n" +
			"  labdash theme preview --icons=ascii\n" +
			"  NO_COLOR=1 labdash theme preview\n\n" +
			"The sheet reflows to the width of the terminal it is run in, down to 50\n" +
			"columns. Resize the terminal and run it again to see the narrow layout.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			overrides, err := parseColorOverrides(colors)
			if err != nil {
				return err
			}

			th, err := theme.New(theme.Options{
				Name:    name,
				Icons:   theme.IconSetting(icons),
				NoColor: noColor,
				Colors:  overrides,
			})
			if err != nil {
				return err
			}

			if width <= 0 {
				width = terminalWidth()
			}

			sheet := preview.Sheet{Theme: th, Width: width}
			fmt.Fprint(cmd.OutOrStdout(), sheet.Render())

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "theme", theme.Default,
		"which shipped theme to render: "+strings.Join(theme.Names(), ", "))
	cmd.Flags().StringVar(&icons, "icons", string(theme.IconsAuto),
		"glyph tier: auto, unicode, ascii. auto reads your locale")
	cmd.Flags().BoolVar(&noColor, "no-color", false,
		"render without colour, as NO_COLOR does")
	cmd.Flags().IntVar(&width, "width", 0,
		"draw at this width instead of the terminal's, for a screenshot or a diff")
	cmd.Flags().StringArrayVar(&colors, "color", nil,
		"replace one token for this run, as token=#RRGGBB. Repeatable. "+
			"The produced contrast ratio is reported, never refused")

	return cmd
}

// parseColorOverrides reads the repeatable --color token=#RRGGBB flag.
func parseColorOverrides(values []string) (map[theme.Token]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	out := make(map[theme.Token]string, len(values))
	for _, v := range values {
		token, value, ok := strings.Cut(v, "=")
		if !ok {
			return nil, fmt.Errorf(
				"--color %q is not recognised. Write it as token=#RRGGBB, "+
					"for example --color status.error=#C0323C", v)
		}
		out[theme.Token(strings.TrimSpace(token))] = strings.TrimSpace(value)
	}
	return out, nil
}

// terminalWidth reads the terminal's width, or returns the reference width when
// stdout is not a terminal — a pipe has no width, and a sheet that guesses 80
// for a `less -R` session is a sheet missing three columns.
func terminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return defaultWidth
	}
	return w
}
