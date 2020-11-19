package secrethub

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/secrethub/secrethub-cli/internals/cli/ui"
	"github.com/secrethub/secrethub-cli/internals/secrethub/command"

	"github.com/secrethub/secrethub-go/internals/api"
	"github.com/secrethub/secrethub-go/pkg/secretpath"

	"github.com/joho/godotenv"
)

// ImportDotEnvCommand handles the migration of secrets from .env files to SecretHub.
type ImportDotEnvCommand struct {
	io          ui.IO
	path        api.DirPath
	interactive bool
	force       bool
	dotenvFile  string
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
	clause := r.Command("dotenv", "Import secrets from `.env` files. Outputs a `secrethub.env` file, containing references to your secrets in SecretHub.")
	clause.Arg("dir-path", "path to a directory on SecretHub in which to store the imported secrets").PlaceHolder(dirPathPlaceHolder).SetValue(&cmd.path)
	clause.Flag("interactive", "Interactive mode. Edit the paths to where the secrets should be written.").Short('i').BoolVar(&cmd.interactive)
	clause.Flag("env-file", "The location of the .env file. Defaults to `.env`.").Default(".env").ExistingFileVar(&cmd.dotenvFile)
	registerForceFlag(clause).BoolVar(&cmd.force)
	command.BindAction(clause, cmd.Run)
}

func (cmd *ImportDotEnvCommand) Run() error {
	envVar, err := godotenv.Read(cmd.dotenvFile)
	if err != nil {
		return err
	}

	client, err := cmd.newClient()
	if err != nil {
		return err
	}

	locationsMap := make(map[string]string)
	for key := range envVar {
		locationsMap[key] = secretpath.Join(cmd.path.Value(), strings.ToLower(key))
	}

	if cmd.interactive {
		mappingString, err := openEditor(buildFile(locationsMap))
		if err != nil {
			return err
		}
		locationsMap = buildMap(mappingString)
	}

	if !cmd.force {
		alreadyExist := make(map[string]struct{})
		var m sync.Mutex
		errGroup, _ := errgroup.WithContext(context.Background())
		for _, path := range locationsMap {
			errGroup.Go(func(path string) func() error {
				return func() error {
					exists, err := client.Secrets().Exists(path)
					if err != nil {
						return err
					}
					if exists {
						m.Lock()
						alreadyExist[path] = struct{}{}
						m.Unlock()
					}
					return nil
				}
			}(path))
		}
		err = errGroup.Wait()
		if err != nil {
			return err
		}

		for path := range alreadyExist {
			confirmed, err := ui.AskYesNo(cmd.io, fmt.Sprintf("A secret at location %s already exists. "+
				"This import process will overwrite this secret. Do you wish to continue?", path), ui.DefaultNo)

			if err != nil {
				return err
			}

			if !confirmed {
				_, err = fmt.Fprintln(cmd.io.Output(), "Aborting.")
				if err != nil {
					return err
				}
				return nil
			}
		}
	}

	errGroup, _ := errgroup.WithContext(context.Background())
	for envVarKey, secretPath := range locationsMap {
		errGroup.Go(func(envVarKey, secretPath string) func() error {
			return func() error {
				envVarValue, ok := envVar[envVarKey]
				if !ok {
					return fmt.Errorf("key not found in .env file: %s", envVarKey)
				}

				err = client.Dirs().CreateAll(secretpath.Parent(secretPath))
				if err != nil {
					return fmt.Errorf("creating parent directories for %s: %s", secretPath, err)
				}

				_, err = client.Secrets().Write(secretPath, []byte(envVarValue))
				if err != nil {
					return err
				}

				return nil
			}
		}(envVarKey, secretPath))
	}
	err = errGroup.Wait()
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(cmd.io.Output(), "Transfer complete! The secrets have been written to %s.\n", cmd.path.String())
	if err != nil {
		return err
	}

	return nil
}

// openEditor opens an editor with the provided input as contents,
// lets the user edit those contents with the editor and returns
// the edited contents.
// Note that this functions is blocking for user input.
func openEditor(input string) (string, error) {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "secrethub-")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	_, err = tmpFile.WriteString(input)
	if err != nil {
		return "", err
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "editor"
	}

	cmd := exec.Command(editor, tmpFile.Name())

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	if err != nil {
		return "", err
	}
	err = cmd.Wait()
	if err != nil {
		return "", err
	}

	out, err := ioutil.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}

	return string(out), nil
}

func buildFile(locationsMap map[string]string) string {
	output := "Choose the paths to where your secrets will be written:\n"

	for envVarKey, secretPath := range locationsMap {
		output += fmt.Sprintf("%s => %s\n", envVarKey, secretPath)
	}
	return output
}

func buildMap(input string) map[string]string {
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Scan()
	locationsMap := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()
		split := strings.Split(line, "=>")
		locationsMap[strings.TrimSpace(split[0])] = strings.TrimSpace(split[1])
	}
	return locationsMap
}
