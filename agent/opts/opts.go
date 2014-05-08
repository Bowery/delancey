// Copyright 2014 Bowery, Inc.
package opts

import (
	"flag"
	"os"
)

var (
	cwd, _    = os.Getwd()
	TargetDir = flag.String("dir", cwd, "Directory to sync files to.")
)

func init() {
	flag.Parse()
}
