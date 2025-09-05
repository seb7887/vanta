package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCommand(version, commit, buildTime string) *cobra.Command {
	var detailed bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  `Display version, build information, and runtime details.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if detailed {
				printDetailedVersion(version, commit, buildTime)
			} else {
				printSimpleVersion(version, commit, buildTime)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "Show detailed version information")

	return cmd
}

func printSimpleVersion(version, commit, buildTime string) {
	fmt.Printf("vanta version %s (commit: %s, built: %s)\n", version, commit, buildTime)
}

func printDetailedVersion(version, commit, buildTime string) {
	fmt.Printf("Vanta - High-performance OpenAPI Mock Server\n")
	fmt.Printf("\n")
	fmt.Printf("Version:      %s\n", version)
	fmt.Printf("Git Commit:   %s\n", commit)
	fmt.Printf("Built:        %s\n", buildTime)
	fmt.Printf("Go Version:   %s\n", runtime.Version())
	fmt.Printf("OS/Arch:      %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Compiler:     %s\n", runtime.Compiler)
}