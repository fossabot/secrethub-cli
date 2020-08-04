package secrethub

import (
	"encoding/hex"
	"time"

	"github.com/secrethub/secrethub-cli/internals/cli/clip"
	"github.com/secrethub/secrethub-cli/internals/cli/cloneproc"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

// defaultClearClipboardAfter defines the default TTL for data written to the clipboard.
const defaultClearClipboardAfter = 45 * time.Second

// ClearClipboardCommand is a command to clear the contents of the clipboard after some time passed.
type ClearClipboardCommand struct {
	clipper clip.Clipper
	hash    []byte
	timeout time.Duration
}

// NewClearClipboardCommand creates a new ClearClipboardCommand.
func NewClearClipboardCommand() *ClearClipboardCommand {
	return &ClearClipboardCommand{
		clipper: clip.NewClipboard(),
	}
}

func (cmd *ClearClipboardCommand) Register(c *cobra.Command) {
	command := &cobra.Command{
		Use:   "clipboard-clear",
		Short: "Removes secret from clipboard.",
		RunE:  cmd.Run,
	}

	command.Flags().BytesHexVar(&cmd.hash, "hash", []byte{}, "Hash from the secret to be cleared")
	command.Flags().DurationVar(&cmd.timeout, "timeout", 0, "Time to wait before clearing in seconds")

	c.AddCommand(command)
}

// Run handles the command with the options as specified in the command.
func (cmd *ClearClipboardCommand) Run(_ *cobra.Command, args []string) error {
	if cmd.timeout > 0 {
		time.Sleep(cmd.timeout)
	}

	read, err := cmd.clipper.ReadAll()
	if err != nil {
		return err
	}

	err = bcrypt.CompareHashAndPassword(cmd.hash, read)
	if err != nil {
		return nil
	}

	err = cmd.clipper.WriteAll(nil)
	if err != nil {
		return err
	}
	return nil
}

// WriteClipboardAutoClear writes data to the clipboard and clears it after the timeout.
func WriteClipboardAutoClear(data []byte, timeout time.Duration, clipper clip.Clipper) error {
	hash, err := bcrypt.GenerateFromPassword(data, bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	err = clipper.WriteAll(data)
	if err != nil {
		return err
	}

	err = cloneproc.Spawn(
		"clipboard-clear", hex.EncodeToString(hash),
		"--timeout", timeout.String())

	return err
}
