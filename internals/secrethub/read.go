package secrethub

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/docker/go-units"
	"github.com/secrethub/secrethub-cli/internals/cli/clip"
	"github.com/secrethub/secrethub-cli/internals/cli/filemode"
	"github.com/secrethub/secrethub-cli/internals/cli/posix"
	"github.com/secrethub/secrethub-cli/internals/cli/ui"
	"github.com/secrethub/secrethub-go/internals/api"
	"github.com/spf13/cobra"
)

// ReadCommand is a command to read a secret.
type ReadCommand struct {
	io                  ui.IO
	path                api.SecretPath
	useClipboard        bool
	clearClipboardAfter time.Duration
	clipper             clip.Clipper
	outFile             string
	fileMode            string
	noNewLine           bool
	newClient           newClientFunc
}

// NewReadCommand creates a new ReadCommand.
func NewReadCommand(io ui.IO, newClient newClientFunc) *ReadCommand {
	return &ReadCommand{
		clipper:             clip.NewClipboard(),
		clearClipboardAfter: 45 * time.Second,
		io:                  io,
		newClient:           newClient,
	}
}

// Register initializes the command with the execution function and the flags.
func (cmd *ReadCommand) Register(c *cobra.Command) {
	var command = &cobra.Command{
		Use:   "read",
		Short: "Read a secret.",
		Args:  cobra.ExactValidArgs(1),
		RunE:  cmd.Run,
	}

	command.Flags().BoolVarP(&cmd.useClipboard, "clip", "c", false, fmt.Sprintf(
		"Copy the secret value to the clipboard. The clipboard is automatically cleared after %s.",
		units.HumanDuration(cmd.clearClipboardAfter),
	))
	command.Flags().StringVarP(&cmd.outFile, "out-file", "o", "", "Write the secret value to this file.")
	command.Flags().StringVar(&cmd.fileMode, "file-mode", "0600", "Set filemode for the output file. Defaults to 0600 (read and write for current user) and is ignored without the --out-file flag.")
	command.Flags().BoolVarP(&cmd.noNewLine, "no-newline", "n", false, "Do not print a new line after the secret.")
	c.AddCommand(command)
}

// Run handles the command with the options as specified in the command.
func (cmd *ReadCommand) Run(command *cobra.Command, args []string) error {
	client, err := cmd.newClient()
	if err != nil {
		return err
	}

	secret, err := client.Secrets().Versions().GetWithData(args[0])
	if err != nil {
		return err
	}

	if cmd.useClipboard {
		err = WriteClipboardAutoClear(secret.Data, cmd.clearClipboardAfter, cmd.clipper)
		if err != nil {
			return err
		}

		fmt.Fprintf(
			cmd.io.Output(),
			"Copied %s to clipboard. It will be cleared after %s.\n",
			cmd.path,
			units.HumanDuration(cmd.clearClipboardAfter),
		)
	}

	secretData := secret.Data
	if !cmd.noNewLine {
		secretData = posix.AddNewLine(secretData)
	}

	if cmd.outFile != "" {
		fileMode, _ := filemode.Parse(cmd.fileMode)
		err = ioutil.WriteFile(cmd.outFile, secretData, fileMode.FileMode())
		if err != nil {
			return ErrCannotWrite(cmd.outFile, err)
		}
	}

	if cmd.outFile == "" && !cmd.useClipboard {
		fmt.Fprintf(cmd.io.Output(), "%s", string(secretData))
	}

	return nil
}
