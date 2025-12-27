package main

import (
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for vrclog.

To load completions:

Bash:
  $ source <(vrclog completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ vrclog completion bash > /etc/bash_completion.d/vrclog
  # macOS:
  $ vrclog completion bash > $(brew --prefix)/etc/bash_completion.d/vrclog

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ vrclog completion zsh > "${fpath[1]}/_vrclog"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ vrclog completion fish | source

  # To load completions for each session, execute once:
  $ vrclog completion fish > ~/.config/fish/completions/vrclog.fish

PowerShell:
  PS> vrclog completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> vrclog completion powershell > vrclog.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := cmd.Root()
		out := cmd.OutOrStdout()

		switch args[0] {
		case "bash":
			return root.GenBashCompletionV2(out, true)
		case "zsh":
			return root.GenZshCompletion(out)
		case "fish":
			return root.GenFishCompletion(out, true)
		case "powershell":
			return root.GenPowerShellCompletionWithDesc(out)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
