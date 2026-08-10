package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/library"
	"github.com/irasikhin/praktik/internal/project"
	"github.com/irasikhin/praktik/internal/sync"
)

// loadState loads the manifest + lock for an existing project.
func loadState(p *project.Project) (*project.Manifest, *project.Lock, error) {
	m, err := project.LoadManifest(p.ManifestPath())
	if err != nil {
		if errors.Is(err, project.ErrNoManifest) {
			return nil, nil, &UserError{Msg: "no praktik.toml found — run 'praktik init' first"}
		}
		return nil, nil, &UserError{Msg: err.Error()}
	}
	lock, err := project.LoadLock(p.LockPath())
	if err != nil {
		return nil, nil, &UserError{Msg: err.Error()}
	}
	return m, lock, nil
}

// sourceRepoRef picks the repo/ref for one source: a global --source/--ref
// override applies only when the manifest has a single source (else it would be
// ambiguous).
func sourceRepoRef(c *ctx, m *project.Manifest, s project.Source) (repo, ref string) {
	repo, ref = s.Repo, s.Ref
	if len(m.Sources) == 1 {
		if c.source != "" {
			repo = c.source
		}
		if c.ref != "" {
			ref = c.ref
		}
	}
	return repo, ref
}

// resolveSources resolves every manifest source to its checkout, discovers its
// skills, and selects the chosen set, collecting unmatched selectors as Missing.
// A cached git source is used as-is unless c.refresh is set (`praktik update`),
// which force-fetches it first.
func resolveSources(c *ctx, m *project.Manifest) ([]sync.ResolvedSource, []sync.Missing, error) {
	if c.source != "" && len(m.Sources) > 1 {
		return nil, nil, &UserError{Msg: "--source is ambiguous with multiple [[source]] entries — edit praktik.toml or use `praktik source add`"}
	}
	checkout := library.ResolveCheckout
	if c.refresh {
		checkout = library.RefreshCheckout
	}
	var resolved []sync.ResolvedSource
	var missing []sync.Missing
	for _, s := range m.Sources {
		repo, ref := sourceRepoRef(c, m, s)
		ck, err := checkout(repo, ref)
		if err != nil {
			return nil, nil, &UserError{Msg: fmt.Sprintf("source %s: %s", s.Key(), err)}
		}
		disc, err := ck.Discover(s.Root)
		if err != nil {
			return nil, nil, &UserError{Msg: fmt.Sprintf("source %s: %s", s.Key(), err)}
		}
		chosen, miss, err := ck.Select(s.Root, disc, s.Skills)
		if err != nil {
			return nil, nil, &UserError{Msg: fmt.Sprintf("source %s: %s", s.Key(), err)}
		}
		resolved = append(resolved, sync.ResolvedSource{
			Key: s.Key(), Repo: s.Repo, Ref: s.Ref, Commit: ck.Commit(), Skills: chosen,
		})
		for _, mm := range miss {
			missing = append(missing, sync.Missing{Name: mm, Source: s.Key()})
		}
	}
	return resolved, missing, nil
}

// buildRequest assembles a sync.Request from the manifest's resolved sources.
func buildRequest(c *ctx, p *project.Project, m *project.Manifest, lock *project.Lock) (sync.Request, error) {
	sources, missing, err := resolveSources(c, m)
	if err != nil {
		return sync.Request{}, err
	}
	return sync.Request{
		Sources: sources,
		Targets: m.Targets,
		Root:    p.Root(),
		Lock:    lock,
		Missing: missing,
	}, nil
}

// planFor loads state and computes the reconcile plan without applying — the
// read-only path behind `status` and `diff`.
func planFor(c *ctx, p *project.Project) (sync.Request, *sync.Plan, error) {
	m, lock, err := loadState(p)
	if err != nil {
		return sync.Request{}, nil, err
	}
	req, err := buildRequest(c, p, m, lock)
	if err != nil {
		return sync.Request{}, nil, err
	}
	plan, err := sync.Compute(req)
	if err != nil {
		return sync.Request{}, nil, &UserError{Msg: err.Error()}
	}
	return req, plan, nil
}

// runSync plans and applies the reconcile for a project whose manifest m is
// already loaded (and possibly just mutated), then prints a one-line summary.
// Patterns, when given, scope the reconcile (`praktik update <patterns>`); every
// skill outside the scope is frozen.
func runSync(c *ctx, p *project.Project, m *project.Manifest, patterns ...string) error {
	lock, err := project.LoadLock(p.LockPath())
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	req, err := buildRequest(c, p, m, lock)
	if err != nil {
		return err
	}
	scopeRequest(c, &req, patterns)
	plan, err := sync.Compute(req)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	if err := sync.Apply(req, plan, p.LockPath()); err != nil {
		return &UserError{Msg: err.Error()}
	}
	docs, err := project.SyncDocs(p.Root(), plan.NextLock.Managed())
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	return reportSync(c, plan, docs)
}

// scopeRequest restricts the reconcile to the skills matching patterns
// (axis/glob/name, the usual grammar), matched against the union of desired and
// locked names — so a stale locked skill no source provides can still be pruned
// by name. Patterns matching nothing are warned about on stderr; no patterns
// leaves the request unscoped.
func scopeRequest(c *ctx, req *sync.Request, patterns []string) {
	if len(patterns) == 0 {
		return
	}
	names := scopeNames(req)
	for _, pat := range patterns {
		if len(library.Expand(names, []string{pat})) == 0 {
			fmt.Fprintln(c.errw, color.Yellow(fmt.Sprintf("warning: %q matched no desired or locked skill — nothing to update for it", pat)))
		}
	}
	req.Scope = sync.ScopeTo(library.Expand(names, patterns)...)
}

// scopeNames is the pattern-matching universe for a scoped update: every
// desired skill name plus every currently locked one.
func scopeNames(req *sync.Request) []string {
	set := map[string]bool{}
	for _, src := range req.Sources {
		for _, s := range src.Skills {
			set[s.Name] = true
		}
	}
	for _, name := range req.Lock.Managed() {
		set[name] = true
	}
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

type syncSummary struct {
	Installed int      `json:"installed"`
	Updated   int      `json:"updated"`
	Pruned    int      `json:"pruned"`
	Missing   []string `json:"missing"`
	Docs      []string `json:"docs"`
}

// reportSync prints the post-apply summary (including any rewritten
// praktik:skills doc blocks) and warns about unmatched selectors.
func reportSync(c *ctx, plan *sync.Plan, docs []string) error {
	missing := make([]string, 0, len(plan.Missing))
	for _, m := range plan.Missing {
		fmt.Fprintln(c.errw, color.Yellow(fmt.Sprintf("warning: %q (source %s) matched no skills — kept in the manifest", m.Name, m.Source)))
		missing = append(missing, m.Name)
	}
	in, up, pr := plan.Count(sync.Install), plan.Count(sync.Update), plan.Count(sync.Prune)
	if c.json {
		enc := json.NewEncoder(c.out)
		enc.SetIndent("", "  ")
		return enc.Encode(syncSummary{Installed: in, Updated: up, Pruned: pr,
			Missing: missing, Docs: append([]string{}, docs...)})
	}
	if in+up+pr == 0 && len(docs) == 0 {
		fmt.Fprintln(c.out, color.Faint("=ok — already in sync"))
		return nil
	}
	if in+up+pr > 0 {
		fmt.Fprintf(c.out, "%s %s %s\n",
			color.Green(fmt.Sprintf("+%d", in)),
			color.Yellow(fmt.Sprintf("~%d", up)),
			color.Red(fmt.Sprintf("-%d", pr)))
	}
	for _, d := range docs {
		fmt.Fprintf(c.out, "%s %s %s\n", color.Yellow("~"), d, color.Faint("praktik:skills block"))
	}
	return nil
}
