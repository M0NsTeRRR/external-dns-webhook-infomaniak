package infomaniak

type Config struct {
	APIToken string `env:"INFOMANIAK_API_TOKEN,notEmpty"`
	DryRun   bool   `env:"DRY_RUN" envDefault:"false"`
}
