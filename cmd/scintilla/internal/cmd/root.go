// Copyright 2026 Bellus Development Team - All Rights Reserved

package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

func Root() *cobra.Command {
	var long strings.Builder
	long.WriteString("Scripting infrastructure, based on WebAssembly and C++. ")
	long.WriteString("This development tool provides a set of methods to compile, pack/unpack and")
	long.WriteString(" generate compilation database in Scintilla packages.")
	root := &cobra.Command{
		Use:   "scintilla",
		Short: "Scripting infrastructure, based on WebAssembly and C++",
		Long:  long.String(),
	}
	root.AddCommand(Build(), Pack())
	return root
}
