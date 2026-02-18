package infomaniak

type Config struct {
	APIToken string `env:"INFOMANIAK_API_TOKEN,notEmpty"`
	Debug    bool   `env:"INFOMANIAK_DEBUG" envDefault:"false"`
	DryRun   bool   `env:"DRY_RUN" envDefault:"false"`
}
