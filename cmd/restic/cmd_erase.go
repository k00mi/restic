package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var cmdErase = &cobra.Command{
	Use:   "erase [flags] PATH",
	Short: "Remove a path from an existing snapshot",
	Long: `
blub
`,
	DisableAutoGenTag: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runErase(eraseOptions, globalOptions, args)
	},
}

func init() {
	cmdRoot.AddCommand(cmdErase)

	f := cmdErase.Flags()
	f.StringVar(&eraseOptions.Snapshot, "snapshot", "", "the snapshot")
}

type EraseOptions struct {
	Snapshot string
}

var eraseOptions EraseOptions

func runErase(opts EraseOptions, gopts GlobalOptions, args []string) error {
	fmt.Print("Hello", args)

	return nil
}

// repo, err := OpenRepository(gopts)
// if err != nil {
// 	return err
// }

// lock, err := lockRepoExclusive(repo)
// defer unlockRepo(lock)
// if err != nil {
// 	return err
// }
