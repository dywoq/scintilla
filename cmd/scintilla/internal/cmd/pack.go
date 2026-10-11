// Copyright 2026 dywoq - Apache License 2.0
// https://github.com/dywoq/scintilla

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dywoq/scintilla"
	"github.com/spf13/cobra"
)

func Pack() *cobra.Command {
	pack := &cobra.Command{
		Use:   "pack [package_dir]",
		Short: "Pack your Scintilla package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := scintilla.NewPackage(args[0])
			if err := p.ParseConfig(); err != nil {
				return fmt.Errorf("parsing config failed: %v", err)
			}
			buildModeFlag, _ := cmd.Flags().GetString("build_mode")
			p.BuildMode = scintilla.PackageBuildMode(buildModeFlag)
			env := scintilla.NewPackageEnv(p.Dir, filepath.Join(p.Dir, ".scintilla"))
			env.MainPackage = p
			if err := env.BuildDependencies(); err != nil {
				return fmt.Errorf("building dependencies failed: %v", err)
			}
			if err := env.UnpackDependencies(); err != nil {
				return fmt.Errorf("unpacking dependencies failed: %v", err)
			}
			if err := p.Pack(); err != nil {
				return fmt.Errorf("packing the package failed: %v", err)
			}
			return nil
		},
	}
	pack.Flags().String("build_mode", "debug", "package's build mode")
	return pack
}
