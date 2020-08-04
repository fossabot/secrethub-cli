package secrethub

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/secrethub/secrethub-cli/internals/cli/ui"
	"github.com/secrethub/secrethub-go/internals/api"
	"github.com/spf13/cobra"
)

// RepoLSCommand lists repositories.
type RepoLSCommand struct {
	useTimestamps bool
	quiet         bool
	workspace     api.Namespace
	io            ui.IO
	timeFormatter TimeFormatter
	newClient     newClientFunc
}

// NewRepoLSCommand creates a new RepoLSCommand.
func NewRepoLSCommand(io ui.IO, newClient newClientFunc) *RepoLSCommand {
	return &RepoLSCommand{
		io:        io,
		newClient: newClient,
	}
}

// Register registers the command, arguments and flags on the provided Registerer.
func (cmd *RepoLSCommand) Register(c *cobra.Command) {
	command := &cobra.Command{
		Use:     "ls",
		Short:   "List all repositories you have access to.",
		Aliases: []string{"list"},
		Args:    cobra.ExactValidArgs(1),
		RunE:    cmd.Run,
	}
	command.Flags().BoolVarP(&cmd.quiet, "quiet", "q", false, "Only print paths.")
	c.AddCommand(command)
}

// Run lists the repositories a user has access to.
func (cmd *RepoLSCommand) Run(_ *cobra.Command, args []string) error {
	cmd.timeFormatter = NewTimeFormatter(cmd.useTimestamps)

	client, err := cmd.newClient()
	if err != nil {
		return err
	}

	var list []*api.Repo
	if args[0] == "" {
		list, err = client.Repos().ListMine()
		if err != nil {
			return err
		}
	} else {
		list, err = client.Repos().List(args[0])
		if err != nil {
			return err
		}
	}

	sort.Sort(api.SortRepoByName(list))

	if cmd.quiet {
		for _, repo := range list {
			fmt.Fprintf(cmd.io.Output(), "%s\n", repo.Path())
		}
	} else {
		w := tabwriter.NewWriter(cmd.io.Output(), 0, 2, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\n", "NAME", "STATUS", "CREATED")
		for _, repo := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\n", repo.Path(), repo.Status, cmd.timeFormatter.Format(repo.CreatedAt.Local()))
		}
		err = w.Flush()
		if err != nil {
			return err
		}
	}

	return nil
}
