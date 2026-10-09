// Copyright 2026 Bellus Development Team - All Rights Reserved

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/bellus-team/scintilla"
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
			env := scintilla.NewPackageEnv(p.Dir, filepath.Join(p.Dir, ".scintilla"))
			env.MainPackage = p
			if err := env.UnpackDependencies(); err != nil {
				return fmt.Errorf("unpacking dependencies failed: %v", err)
			}
			if err := p.GenerateCompilationDatabase(); err != nil {
				return fmt.Errorf("generating compilation database in the package failed: %v", err)
			}
			return nil
		},
	}
	return compiledb
}
