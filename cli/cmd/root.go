package cmd

import "github.com/spf13/cobra"

// RunCommand returns the `run <image>` command for mounting under the unified
// `workshop` umbrella.
func RunCommand() *cobra.Command {
	return runCmd
}
