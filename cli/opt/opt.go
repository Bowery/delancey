package opt

import (
	"flag"
)

// TODO (thebyrd) generate this from All()
var (
	AgentAddr = flag.String("addr", "", "Address of Agent.")
	StartCmd  = flag.String("start", "", "Command to start application.")
	TestCmd   = flag.String("test", "", "Command to test application.")
	BuildCmd  = flag.String("build", "", "Command to build application. Often used to install dependencies. It's always run before the Start Command.")
	Path      = flag.String("path", ".", "Path to sync files from.")
)

type Option struct {
	Name        string
	Default     string
	Description string
}

func init() {
	flag.Parse()
}

func All() []Option {
	return []Option{
		Option{"addr", "", "Address of Agent."},
		Option{"start", "", "Command to start application."},
		Option{"test", "", "Command to test application."},
		Option{"build", "", "Command to build application. Often used to install dependencies. It's always run before the Start Command."},
		Option{"path", ".", "Path to sync files from."},
	}
}
