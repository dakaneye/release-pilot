package main

import (
	"fmt"
	"os"

	"github.com/dakaneye/release-pilot/internal/ship"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:   "release-pilot",
		Short: "Orchestrate releases with AI-powered release notes",
	}

	root.AddCommand(versionCmd())
	root.AddCommand(shipCmd())
	root.AddCommand(initCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	}
}

func shipCmd() *cobra.Command {
	var opts ship.Options

	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Run the release pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}
			return ship.Run(cmd.Context(), dir, opts)
		},
	}

	cmd.Flags().StringVar(&opts.Step, "step", "", "run a single pipeline step")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "preview without making changes")
	cmd.Flags().BoolVar(&opts.Sign, "sign", false, "enable cosign signing")
	cmd.Flags().StringVar(&opts.VersionOver, "version", "", "override the version (e.g. v1.2.3)")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "reset pipeline state and re-run")
	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "path to config file")

	return cmd
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a default .release-pilot.yaml config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			const defaultConfig = `# release-pilot configuration
# See: https://github.com/dakaneye/release-pilot

ecosystem: auto
model: claude-sonnet-4-6

notes:
  include-diffs: false

github:
  draft: false
  prerelease: false
`
			path := ".release-pilot.yaml"
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("%s already exists", path)
			}
			if err := os.WriteFile(path, []byte(defaultConfig), 0o644); err != nil {
				return fmt.Errorf("write config: %w", err)
			}
			fmt.Printf("created %s\n", path)
			return nil
		},
	}
}
