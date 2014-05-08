// Copyright 2013-2014 Bowery, Inc.
package cmds

import (
	"Bowery/crosswalk/cli/opt"
	"fmt"
	"os"
	"text/tabwriter"
)

func init() {
	Cmds["help"] = &Cmd{helpRun, "help", "Display usage for commands."}
}

func helpRun(args ...string) int {
	// Ensure output is correctly aligned.
	tabWriter := tabwriter.NewWriter(os.Stderr, 0, 0, 8, ' ', 0)
	fmt.Fprintln(os.Stderr, "Usage: crosswalk [options]\n\nOptions:")

	for _, cmd := range opt.All() {
		// \t is used to separate columns.
		fmt.Fprintln(tabWriter, "  --"+cmd.Name+"\t"+cmd.Description)
	}
	tabWriter.Flush()
	return 2 // --help uses 2.
}
