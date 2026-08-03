package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/giancarlosisasi/labdash/internal/keymap"
)

func newKeysCmd() *cobra.Command {
	var (
		list     bool
		markdown bool
	)

	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Print the keymap",
		Long: "Print the keymap.\n\n" +
			"Keys are fixed: every action has one binding, in every view, on every\n" +
			"platform. There is nothing to rebind, so this is a reference rather than\n" +
			"a starting point for a settings file.\n\n" +
			"  labdash keys --list\n" +
			"      every action name, one per line, for a bug report or a proposal to cite\n\n" +
			"  labdash keys --markdown > cheatsheet.md\n" +
			"      the whole keymap as markdown; this is also what the /keys/ page is\n\n" +
			"Both print the whole table, including actions a later release brings. The\n" +
			"help overlay inside the application shows the subset this build does.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if list && markdown {
				return errors.New("--list and --markdown print the same keymap two ways; pick one")
			}
			if markdown {
				fmt.Fprint(cmd.OutOrStdout(), keymap.Markdown())
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), keymap.List())
			return nil
		},
	}

	cmd.Flags().BoolVar(&list, "list", false,
		"print every action name, one per line (the default)")
	cmd.Flags().BoolVar(&markdown, "markdown", false,
		"print the whole keymap as markdown, exactly as the /keys/ page carries it")

	return cmd
}
