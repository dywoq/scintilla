// Copyright 2026 Bellus Development Team - All Rights Reserved

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/bellus-team/scintilla"
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
			env := scintilla.NewPackageEnv(p.Dir, filepath.Join(p.Dir, ".scintilla"))
			env.MainPackage = p
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
	return build
}
