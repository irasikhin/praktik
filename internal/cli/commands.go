package cli

import "github.com/alecthomas/kong"

// praktikCLI is the root kong struct. Global flags live at this level (kong treats
// them as persistent); subcommands are nested via `cmd:""`.
type praktikCLI struct {
	// --- global flags ---
	Source  string           `help:"Path to the praktik repo / its skills/ dir, or a git remote cloned on demand (default: $PRAKTIK_DIR, else an ancestor of the cwd)." placeholder:"SRC"`
	Ref     string           `help:"Git branch or tag for a remote --source (default: the manifest's, else main)." placeholder:"REF"`
	Global  bool             `help:"Operate on the global scope ($HOME targets, ~/.config/praktik/config.toml)." short:"g"`
	JSON    bool             `help:"Machine-readable JSON output."`
	NoColor bool             `help:"Disable colored output." short:"C"`
	Version kong.VersionFlag `short:"V" help:"Print version and exit."`

	// --- consumption: declarative vendoring lifecycle ---
	Init    InitCmd    `cmd:"" help:"Scaffold praktik.toml and vendor the named skills."`
	Add     AddCmd     `cmd:"" help:"Add skill patterns to the manifest and sync."`
	Rm      RmCmd      `cmd:"" help:"Remove skill patterns from the manifest and sync."`
	Sync    SyncCmd    `cmd:"" help:"Reconcile every target to the manifest."`
	Update  UpdateCmd  `cmd:"" help:"Re-resolve skills from source and rewrite the lock."`
	Status  StatusCmd  `cmd:"" help:"Show drift: +added ~changed -pruned, or =ok."`
	Diff    DiffCmd    `cmd:"" help:"Diff a vendored copy against its source."`
	Src     SourceCmd  `cmd:"" name:"source" help:"Manage the skill sources (git repos / local paths)."`
	Target  TargetCmd  `cmd:"" help:"Manage the agent target set."`
	Migrate MigrateCmd `cmd:"" help:"Import an existing .skillshare/ setup into praktik.toml."`

	// --- authoring & discovery ---
	List       ListCmd       `cmd:"" help:"List available skills, grouped by axis."`
	Search     SearchCmd     `cmd:"" help:"Search skills by name, description, or axis."`
	Show       ShowCmd       `cmd:"" help:"Show a skill's frontmatter and body."`
	Pick       PickCmd       `cmd:"" help:"Interactively pick a skill (fzf); show or --add it."`
	Doctor     DoctorCmd     `cmd:"" help:"Sanity-check the project: source, targets, lock, leftovers."`
	New        NewCmd        `cmd:"" help:"Scaffold a new skills/<name>/SKILL.md template."`
	Validate   ValidateCmd   `cmd:"" help:"Lint every skill's SKILL.md frontmatter."`
	Handbook   HandbookCmd   `cmd:"" help:"Regenerate HANDBOOK.md from skill frontmatter."`
	Completion CompletionCmd `cmd:"" help:"Generate a shell completion script (bash, zsh, fish)."`
}
