package secrethub

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/secrethub/secrethub-cli/internals/cli/ui"
	"github.com/secrethub/secrethub-cli/internals/secrethub/command"
	"github.com/secrethub/secrethub-go/internals/api"
)

// ImportDotEnvCommand handles the migration of secrets from .env files to SecretHub.
type ImportDotEnvCommand struct {
	io          ui.IO
	path        api.DirPath
	interactive bool
	dotenv      string
	editor      string
	newClient   newClientFunc
}

// NewImportDotEnvCommand creates a new ImportDotEnvCommand.
func NewImportDotEnvCommand(io ui.IO, newClient newClientFunc) *ImportDotEnvCommand {
	return &ImportDotEnvCommand{
		io:        io,
		newClient: newClient,
	}
}

// Register registers the command and its sub-commands on the provided Registerer.
func (cmd *ImportDotEnvCommand) Register(r command.Registerer) {
	clause := r.Command("dot-env", "Import secrets from .env files.")
	clause.Arg("dir-path", "The path to where to write the new secrets").PlaceHolder(dirPathPlaceHolder).SetValue(&cmd.path)
	clause.Flag("interactive", "Interactive mode. Edit the paths to where the secrets should be written.").Short('i').BoolVar(&cmd.interactive)
	clause.Flag("env-file", "The location of the .env file. Defaults to `.env`.").Default(".env").ExistingFileVar(&cmd.dotenv)
	clause.Flag("editor", "The editor where you will define your secret paths. Only available in the interactive mode.").Default("nano").HintOptions("vim", "nano").StringVar(&cmd.editor)
	command.BindAction(clause, cmd.Run)
}

func (cmd *ImportDotEnvCommand) Run() error {
	var envVar map[string]string
	envVar, err := godotenv.Read()
	if err != nil {
		return err
	}

	client, err := cmd.newClient()
	if err != nil {
		return err
	}

	if cmd.interactive {
		locationsMap, err := openEditor(cmd.editor, cmd.path.Value(), getMapKeys(envVar))
		if err != nil {
			return err
		}

		for key, value := range envVar {
			_, err := client.Secrets().Write(locationsMap[key], []byte(value))
			if err != nil {
				return err
			}
		}
	} else {
		for key, value := range envVar {
			_, err := client.Secrets().Write(cmd.path.Value()+"/"+key, []byte(value))
			if err != nil {
				return err
			}
		}
	}
	_, err = fmt.Fprintf(cmd.io.Output(), "Transfer complete! The secrets have been written to %s.\n", cmd.path.String())
	if err != nil {
		return err
	}

	return nil
}

func getMapKeys(stringMap map[string]string) []string {
	keys := make([]string, 0, len(stringMap))
	for k := range stringMap {
		keys = append(keys, k)
	}
	return keys
}