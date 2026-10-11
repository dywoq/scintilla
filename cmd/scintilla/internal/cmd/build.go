// Copyright 2026 dywoq - Apache License 2.0
// https://github.com/dywoq/scintilla

package cmd

import (
	"fmt"

	"github.com/dywoq/scintilla"
	"github.com/spf13/cobra"
)

func Build() *cobra.Command {
	build := &cobra.Command{
		Use:   "build [package_dir]",
		Short: "Compile your Scintilla package",
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
			if err := p.Compile(); err != nil {
				return fmt.Errorf("compilation of the package failed: %v", err)
			}
			switch p.Conf.Type {
			case scintilla.PackageTypeLib:
				return p.Archive()
			case scintilla.PackageTypeProgram:
				return p.Link()
			}
			return fmt.Errorf("wrong package type: %q", p.Conf.Type)
		},
	}
	build.Flags().String("build_mode", string(scintilla.PackageBuildModeDebug), "package's build mode")
	return build
}
