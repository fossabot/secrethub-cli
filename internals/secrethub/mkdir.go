package secrethub

import (
	"fmt"
	"os"

	"github.com/secrethub/secrethub-cli/internals/cli/ui"
	"github.com/secrethub/secrethub-go/internals/api"
	"github.com/secrethub/secrethub-go/pkg/secrethub"
	"github.com/spf13/cobra"
)

// Errors
var (
	ErrMkDirOnRootDir = errMain.Code("mkdir_on_root_dir").Error("You cannot create a directory on the repo path. You can create subdirectories :owner/:repo_name/:directory_name.")
)

// MkDirCommand creates a new directory inside a repository.
type MkDirCommand struct {
	io        ui.IO
	paths     dirPathList
	parents   bool
	newClient newClientFunc
}

// NewMkDirCommand returns a new command.
func NewMkDirCommand(io ui.IO, newClient newClientFunc) *MkDirCommand {
	return &MkDirCommand{
		io:        io,
		newClient: newClient,
	}
}

func (cmd *MkDirCommand) Register(c *cobra.Command) {
	command := &cobra.Command{
		Use:   "mkdir",
		Short: "Create a new directory.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  cmd.Run,
	}
	command.Flags().BoolVar(&cmd.parents, "parents", false, "Create parent directories if needed. Does not error when directories already exist.")
	c.AddCommand(command)
}

// Run executes the command.
func (cmd *MkDirCommand) Run(_ *cobra.Command, args []string) error {
	client, err := cmd.newClient()
	if err != nil {
		return err
	}

	cmd.paths = args
	for _, path := range cmd.paths {
		err := cmd.createDirectory(client, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create a new directory at %s: %s\n", path, err)
		} else {
			fmt.Fprintf(cmd.io.Output(), "Created a new directory at %s\n", path)
		}
	}
	return nil
}

// createDirectory validates the given path and creates a directory on it.
func (cmd *MkDirCommand) createDirectory(client secrethub.ClientInterface, path string) error {
	dirPath, err := api.NewDirPath(path)
	if err != nil {
		return err
	}
	if dirPath.IsRepoPath() {
		return ErrMkDirOnRootDir
	}
	if cmd.parents {
		return client.Dirs().CreateAll(dirPath.Value())
	}
	_, err = client.Dirs().Create(dirPath.Value())
	return err
}

// dirPathList represents the value of a repeatable directory path argument.
type dirPathList []string

func (d *dirPathList) String() string {
	return ""
}

func (d *dirPathList) Set(path string) error {
	*d = append(*d, path)
	return nil
}

func (d *dirPathList) IsCumulative() bool {
	return true
}
