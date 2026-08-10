package cli

import "fmt"

// CompletionCmd prints a shell completion script for the top-level commands.
type CompletionCmd struct {
	Shell string `arg:"" enum:"bash,zsh,fish" help:"Shell: bash, zsh, or fish."`
}

func (cmd *CompletionCmd) Run(c *ctx) error {
	switch cmd.Shell {
	case "bash":
		fmt.Fprint(c.out, bashCompletion)
	case "zsh":
		fmt.Fprint(c.out, zshCompletion)
	case "fish":
		fmt.Fprint(c.out, fishCompletion)
	}
	return nil
}

const bashCompletion = `# praktik bash completion
_praktik() {
  local cur cmds
  cur="${COMP_WORDS[COMP_CWORD]}"
  cmds="list show validate handbook completion"
  if [ "${COMP_CWORD}" -eq 1 ]; then
    COMPREPLY=( $(compgen -W "${cmds}" -- "${cur}") )
  fi
}
complete -F _praktik praktik
`

const zshCompletion = `#compdef praktik
_praktik() {
  local -a cmds
  cmds=(list show validate handbook completion)
  if (( CURRENT == 2 )); then
    compadd -- ${cmds}
  fi
}
compdef _praktik praktik
`

const fishCompletion = `# praktik fish completion
complete -c praktik -f
complete -c praktik -n __fish_use_subcommand -a list -d 'List available skills'
complete -c praktik -n __fish_use_subcommand -a show -d 'Show a skill'
complete -c praktik -n __fish_use_subcommand -a validate -d 'Lint skill frontmatter'
complete -c praktik -n __fish_use_subcommand -a handbook -d 'Regenerate HANDBOOK.md'
complete -c praktik -n __fish_use_subcommand -a completion -d 'Shell completion script'
`
