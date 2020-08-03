package secrethub

import (
	"os"

	"github.com/spf13/cobra"
)

// newCompletionCommand returns a command that, when executed, generates a completion script
// for a specific shell, based on the argument it is provided with. It is able to generate
// completions for Bash, ZSh, Fish and PowerShell.
func newCompletionCommand() *cobra.Command {
	var completionCmd = &cobra.Command{
		Use:                   "completion",
		Short:                 "Generate completion script",
		DisableFlagsInUseLine: true,
		Hidden:                true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			switch args[0] {
			case "bash":
				_ = cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				_ = cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				_ = cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				_ = cmd.Root().GenPowerShellCompletion(os.Stdout)
			}
		},
	}
	return completionCmd
}
