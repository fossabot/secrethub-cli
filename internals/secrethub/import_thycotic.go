package secrethub

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/secrethub/secrethub-cli/internals/cli/ui"
	"github.com/secrethub/secrethub-cli/internals/secrethub/command"

	"github.com/secrethub/secrethub-go/pkg/secretpath"
)

// ImportThycoticCommand handles importing secrets from Thycotic.
type ImportThycoticCommand struct {
	io        ui.IO
	newClient newClientFunc
	file      string
}

// NewImportThycoticCommand creates a new ImportThycoticCommand.
func NewImportThycoticCommand(io ui.IO, newClient newClientFunc) *ImportThycoticCommand {
	return &ImportThycoticCommand{
		io:        io,
		newClient: newClient,
	}
}

// Register registers the command and its sub-commands on the provided Registerer.
func (cmd *ImportThycoticCommand) Register(r command.Registerer) {
	clause := r.Command("thycotic", "Import secrets from a Thycotic .csv export file.")
	clause.Arg("file", "Path to .csv export file of your Thycotic secrets.").Required().StringVar(&cmd.file)
	command.BindAction(clause, cmd.Run)
}

func (cmd *ImportThycoticCommand) Run() error {
	if !strings.HasSuffix(cmd.file, ".csv") {
		return fmt.Errorf("currently only .csv files are supported")
	}

	r, err := os.Open(cmd.file)
	if err != nil {
		return fmt.Errorf("could not open file: %s", err)
	}

	csvReader := csv.NewReader(r)
	header, err := csvReader.Read()
	if err != nil {
		return fmt.Errorf("could not read from .csv file: %s", err)
	}

	if len(header) < 1 || header[0] != "SecretName" {
		return fmt.Errorf("first column of .csv file should contain the SecretName")
	}

	if len(header) < 2 || header[1] != "FolderPath" {
		return fmt.Errorf("second column of .csv file should contain the FolderPath")
	}

	if len(header) < 3 {
		return fmt.Errorf("nothing to import; there should be at least one column next to SecretName and FolderPath, that contains secrets to import")
	}

	client, err := cmd.newClient()
	if err != nil {
		return err
	}

	dirs := map[string]struct{}{}
	secrets := map[string][]byte{}

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("could not read record: %s", err)
		}

		dirPath := record[1]
		if strings.ContainsAny(dirPath, "/") {
			return fmt.Errorf("path %s contains a '/' character (forward slash), which is not allowed; paths should be separated with '\\' characters (backslash)", dirPath)
		}
		dirPath = strings.ReplaceAll(dirPath, "\\", "/")

		dirPath = secretpath.Join(dirPath, record[0])

		dirPath = strings.ReplaceAll(dirPath, " ", "_")

		dirs[dirPath] = struct{}{}

		for i, field := range record[1:] {
			secretPath := secretpath.Join(dirPath, strings.ReplaceAll(header[i], " ", "_"))
			_, exists := secrets[secretPath]
			if exists {
				return fmt.Errorf("secret '%s' encountered twice", secretPath)
			}
			secrets[secretPath] = []byte(field)
		}
	}

	fmt.Fprintln(cmd.io.Stdout(), "You're about to create the following resources:")
	fmt.Fprintf(cmd.io.Stdout(), "%d directories:\n\n", len(dirs))
	for dirPath := range dirs {
		fmt.Fprintln(cmd.io.Stdout(), dirPath)
	}

	fmt.Fprintf(cmd.io.Stdout(), "\n%d secrets:\n\n", len(secrets))
	for secretPath := range secrets {
		fmt.Fprintln(cmd.io.Stdout(), secretPath)
	}

	fmt.Fprintln(cmd.io.Stdout(), "")
	confirmed, err := ui.AskYesNo(cmd.io, "Are you sure you want to proceed?", ui.DefaultYes)
	if err != nil {
		return err
	}
	if !confirmed {
		return fmt.Errorf("Aborting...")
	}

	for dirPath := range dirs {
		err = client.Dirs().CreateAll(dirPath)
		if err != nil {
			return fmt.Errorf("could not create directory '%s': %s", dirPath, err)
		}
	}

	for secretPath, value := range secrets {
		_, err = client.Secrets().Write(secretPath, value)
		if err != nil {
			return fmt.Errorf("could not write secret '%s': %s", secretPath, err)
		}
	}

	fmt.Fprintf(cmd.io.Stdout(), "Successfully imported %d directories containing %d secrets\n", len(dirs), len(secrets))

	return nil
}