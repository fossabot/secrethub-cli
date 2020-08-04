package secrethub

import (
	"github.com/secrethub/secrethub-cli/internals/cli/ui"
	"github.com/spf13/cobra"
)

// AccountCommand handles operations on SecretHub accounts.
type AccountCommand struct {
	io              ui.IO
	newClient       newClientFunc
	credentialStore CredentialConfig
}

// NewAccountCommand creates a new AccountCommand.
func NewAccountCommand(io ui.IO, newClient newClientFunc, credentialStore CredentialConfig) *AccountCommand {
	return &AccountCommand{
		io:              io,
		newClient:       newClient,
		credentialStore: credentialStore,
	}
}

// Register initializes the command.
func (cmd *AccountCommand) Register(c *cobra.Command) {
	command := &cobra.Command{
		Use:   "account",
		Short: "Manage your personal account.",
	}

	NewAccountInspectCommand(cmd.io, cmd.newClient).Register(command)
	NewAccountInitCommand(cmd.io, cmd.newClient, cmd.credentialStore).Register(command)
	NewAccountEmailVerifyCommand(cmd.io, cmd.newClient).Register(command)

	c.AddCommand(command)
}
