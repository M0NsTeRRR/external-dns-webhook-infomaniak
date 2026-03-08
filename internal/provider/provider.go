package provider

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/m0nsterrr/external-dns-webhook-infomaniak/internal/config"
	"github.com/m0nsterrr/external-dns-webhook-infomaniak/internal/infomaniak"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/provider"
)

func Init(config config.Config) (provider.Provider, error) {
	var domainFilter *endpoint.DomainFilter

	createMsg := "creating infomaniak provider with "

	if config.RegexDomainFilter != "" {
		createMsg += fmt.Sprintf("regexp domain filter: '%s', ", config.RegexDomainFilter)
		if config.RegexDomainExclusion != "" {
			createMsg += fmt.Sprintf("with exclusion: '%s', ", config.RegexDomainExclusion)
		}
		domainFilter = endpoint.NewRegexDomainFilter(
			regexp.MustCompile(config.RegexDomainFilter),
			regexp.MustCompile(config.RegexDomainExclusion),
		)
	} else {
		if len(config.DomainFilter) > 0 {
			createMsg += fmt.Sprintf("domain filter: '%s', ", strings.Join(config.DomainFilter, ","))
		}
		if len(config.ExcludeDomains) > 0 {
			createMsg += fmt.Sprintf("exclude domain filter: '%s', ", strings.Join(config.ExcludeDomains, ","))
		}
		domainFilter = endpoint.NewDomainFilterWithExclusions(config.DomainFilter, config.ExcludeDomains)
	}

	createMsg = strings.TrimSuffix(createMsg, ", ")
	if strings.HasSuffix(createMsg, "with ") {
		createMsg += "no kind of domain filters"
	}
	slog.Info(createMsg)

	infomaniakConfig := infomaniak.Config{}
	if err := env.Parse(&infomaniakConfig); err != nil {
		return nil, fmt.Errorf("reading infomaniak configuration failed: %v", err)
	}

	return infomaniak.NewInfomaniakProvider(domainFilter, &infomaniakConfig), nil
}
