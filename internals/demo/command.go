package demo

import (
	"github.com/secrethub/secrethub-cli/internals/cli/ui"
	"github.com/spf13/cobra"
)

// Command is a command to run the secrethub example app.
type Command struct {
	io        ui.IO
	newClient newClientFunc
}

// NewCommand creates a new example app command.
func NewCommand(io ui.IO, newClient newClientFunc) *Command {
	return &Command{
		io:        io,
		newClient: newClient,
	}
}

// Register registers the command, arguments and flags on the provided Registerer.
func (cmd *Command) Register(c *cobra.Command) {
	command := &cobra.Command{
		Use:    "demo",
		Short:  "Manage the demo application.",
		Hidden: true,
	}

	NewInitCommand(cmd.io, cmd.newClient).Register(command)
	//cli.NewServeCommand(cmd.io).Register(command)
	c.AddCommand(command)
}
