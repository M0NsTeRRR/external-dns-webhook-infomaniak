package provider

import (
	"fmt"
	"log/slog"
	"regexp"

	"github.com/caarlos0/env/v11"
	"github.com/m0nsterrr/external-dns-webhook-infomaniak/internal/config"
	"github.com/m0nsterrr/external-dns-webhook-infomaniak/internal/infomaniak"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/provider"
)

func Init(config config.Config) (provider.Provider, error) {
	var domainFilter *endpoint.DomainFilter

	if config.RegexDomainFilter != "" {
		domainFilter = endpoint.NewRegexDomainFilter(
			regexp.MustCompile(config.RegexDomainFilter),
			regexp.MustCompile(config.RegexDomainExclusion),
		)
	} else {
		domainFilter = endpoint.NewDomainFilterWithExclusions(config.DomainFilter, config.ExcludeDomains)
	}

	slog.Info("creating infomaniak provider", "regexp_domain_filter", config.RegexDomainFilter, "with_exclusion", config.RegexDomainExclusion, "domain_filter", config.DomainFilter, "exclude_domain_filter", config.ExcludeDomains)

	infomaniakConfig := infomaniak.Config{}
	if err := env.Parse(&infomaniakConfig); err != nil {
		return nil, fmt.Errorf("reading infomaniak configuration failed: %v", err)
	}

	return infomaniak.NewInfomaniakProvider(domainFilter, &infomaniakConfig), nil
}
