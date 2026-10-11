// Copyright 2026 dywoq - Apache License 2.0
// https://github.com/dywoq/scintilla

package cmd

import (
	"fmt"

	"github.com/dywoq/scintilla"
	"github.com/spf13/cobra"
)

func Compiledb() *cobra.Command {
	compiledb := &cobra.Command{
		Use:   "compiledb [package_dir]",
		Short: "Generate a compilation database in your Scintilla package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := scintilla.NewPackage(args[0])
			if err := p.ParseConfig(); err != nil {
				return fmt.Errorf("parsing config failed: %v", err)
			}
			buildModeFlag, _ := cmd.Flags().GetString("build_mode")
			p.BuildMode = scintilla.PackageBuildMode(buildModeFlag)
			env := scintilla.NewPackageEnv(p.Dir)
			env.MainPackage = p
			if err := env.BuildDependencies(); err != nil {
				return fmt.Errorf("building dependencies failed: %v", err)
			}
			if err := env.UnpackDependencies(); err != nil {
				return fmt.Errorf("unpacking dependencies failed: %v", err)
			}
			if err := p.GenerateCompilationDatabase(); err != nil {
				return fmt.Errorf("generating compilation database in the package failed: %v", err)
			}
			return nil
		},
	}
	compiledb.Flags().String("build_mode", "debug", "package's build mode")
	return compiledb
}
