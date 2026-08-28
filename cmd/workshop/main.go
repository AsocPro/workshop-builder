// Command workshop is the unified entrypoint. With no subcommand it runs the
// container serve path (the in-image ENTRYPOINT); subcommands cover the other
// delivery modes.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	backend "github.com/asocpro/workshop-builder/backend"
	"github.com/asocpro/workshop-builder/backend/servecmd"
	"github.com/asocpro/workshop-builder/backend/setup"
	clicmd "github.com/asocpro/workshop-builder/cli/cmd"
	"github.com/asocpro/workshop-builder/pkg/workshop/compilecli"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "workshop",
		Short:         "Workshop platform — build and run technical Linux workshops",
		SilenceUsage:  true,
		SilenceErrors: true,
		// No subcommand: container-mode entrypoint (env-configured). Blocks.
		Run: func(cmd *cobra.Command, args []string) {
			backend.RunContainer()
		},
	}

	// serve <dir> — standalone mode.
	var opts backend.StandaloneOptions
	serve := &cobra.Command{
		Use:   "serve <dir>",
		Short: "Compile and serve a workshop source directory (standalone mode)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ServeDir = args[0]
			return backend.RunStandalone(opts)
		},
	}
	serve.Flags().StringVar(&opts.Listen, "listen", "127.0.0.1:8080", "listen address")
	serve.Flags().StringVar(&opts.AuthUser, "auth-user", "", "basic auth username")
	serve.Flags().StringVar(&opts.AuthPasswordFile, "auth-password-file", "", "file containing the basic auth password (alternative: WORKSHOP_AUTH_PASSWORD env)")

	// service install|uninstall — systemd unit. servecmd owns its own flag
	// parsing, so pass raw args straight through.
	service := &cobra.Command{
		Use:                "service install|uninstall [flags]",
		Short:              "Install or uninstall the standalone systemd unit",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return servecmd.RunService(args)
		},
	}

	// compile <dir> — compile to a directory or emit JSON.
	var compileOut string
	compile := &cobra.Command{
		Use:   "compile <dir>",
		Short: "Compile a workshop; write a flat-file dir (--output-dir) or emit JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return compilecli.Run(args[0], compileOut, os.Stdout)
		},
	}
	compile.Flags().StringVar(&compileOut, "output-dir", "", "write the compiled flat-file layout here (default: print JSON to stdout)")

	// setup — pre-apply prior steps' setup (devcontainer skip-ahead).
	var setupDir, setupStep string
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Apply step setup through a target step (skip-ahead)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return setup.RunThrough(setupDir, setupStep)
		},
	}
	setupCmd.Flags().StringVar(&setupDir, "workshop-dir", "/workshop", "compiled workshop directory")
	setupCmd.Flags().StringVar(&setupStep, "step", "", "apply setup through this step ID (inclusive)")

	root.AddCommand(serve, service, compile, setupCmd, clicmd.RunCommand())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
