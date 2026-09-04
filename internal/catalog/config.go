package catalog

// Config is the shared complete [catalog] schema for every catalog consumer.
type Config struct {
	RawCSVPath          string        `toml:"raw_csv_path"`
	ScoresCSVPath       string        `toml:"scores_csv_path"`
	ProviderConfigPath  string        `toml:"provider_config_path"`
	BenchmarkConfigPath string        `toml:"benchmark_config_path"`
	CacheTTL            string        `toml:"cache_ttl"`
	WarnOnStaleScores   bool          `toml:"warn_on_stale_scores"`
	Publish             PublishConfig `toml:"publish"`
}

// PublishConfig is the nested [catalog.publish] schema and resolved artifact paths.
type PublishConfig struct {
	RunTests bool `toml:"run_tests"` // legacy accepted option; publishing verification is mandatory

	Enabled       bool     `toml:"enabled"`
	Schedule      string   `toml:"schedule"`
	Timezone      string   `toml:"timezone"`
	Environment   string   `toml:"environment"`
	Branches      []string `toml:"branches"`
	Mode          string   `toml:"mode"` // "pull-request" | "direct-push"
	AutoMerge     bool     `toml:"auto_merge"`
	MergeMethod   string   `toml:"merge_method"` // "squash" | "merge" | "rebase"
	CommitMessage string   `toml:"commit_message"`
	PRTitle       string   `toml:"pr_title"`
	PRLabels      []string `toml:"pr_labels"`
	RawCSVPath    string   `toml:"-"` // from [catalog].raw_csv_path; blank -> default
}
