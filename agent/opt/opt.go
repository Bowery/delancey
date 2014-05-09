// Copyright 2014 Bowery, Inc.
package opt

import (
	"flag"
	"os"
)

var (
	cwd, _    = os.Getwd()
	TargetDir = flag.String("dir", cwd, "Directory to sync files to.")
	Auth      = flag.String("auth", "", "Password used to authenticate with client.")
)

func init() {
	flag.Parse()
}
