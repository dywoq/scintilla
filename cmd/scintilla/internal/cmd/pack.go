package cmd

import (
	"fmt"

	"github.com/bellus-team/scintilla"
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
			if err := p.Pack(); err != nil {
				return fmt.Errorf("packing the package failed: %v", err)
			}
			return nil
		},
	}
	return pack
}
