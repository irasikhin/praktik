package cli

// SyncCmd reconciles every target to the manifest: install missing skills,
// re-copy drifted ones, prune skills no longer desired. Idempotent.
type SyncCmd struct{}

func (cmd *SyncCmd) Run(c *ctx) error {
	p, err := c.project()
	if err != nil {
		return err
	}
	m, _, err := loadState(p)
	if err != nil {
		return err
	}
	return runSync(c, p, m)
}
