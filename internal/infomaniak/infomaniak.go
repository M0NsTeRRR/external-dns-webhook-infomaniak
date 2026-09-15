package infomaniak

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

// Infomaniak requires a minimum TTL of 60 seconds
const minTTL = 60

// Provider implements the DNS provider for Infomaniak DNS.
type Provider struct {
	provider.BaseProvider
	client       *InfomaniakClient
	dryRun       bool
	domainFilter endpoint.DomainFilterInterface
}

func NewInfomaniakProvider(domainFilter endpoint.DomainFilterInterface, configuration *Config) *Provider {
	return &Provider{
		client:       NewInfomaniakClient(configuration),
		dryRun:       configuration.DryRun,
		domainFilter: domainFilter,
	}
}

// Records returns the list of resource records in all zones.
func (p *Provider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	var endpoints []*endpoint.Endpoint

	// Get all domains
	domains, err := p.client.GetDomains(ctx)
	if err != nil {
		return nil, err
	}

	slog.Debug(fmt.Sprintf("Found %d domains", len(domains)))

	// For each domain, get zones and records
	for _, domain := range domains {
		// Apply domain filter if specified
		if p.domainFilter != nil && !p.domainFilter.Match(domain.Name) {
			slog.Debug(fmt.Sprintf("Skipping domain %s due to domain filter", domain.Name))
			continue
		}

		// Get zones for this domain
		zones, err := p.client.GetDomainZones(ctx, domain.Name)
		if err != nil {
			slog.Warn(err.Error())
			continue
		}

		for _, zone := range zones {
			// Get records for this zone
			records, err := p.client.GetRecords(ctx, zone.FQDN)
			if err != nil {
				slog.Warn(err.Error())
				continue
			}

			slog.Debug(fmt.Sprintf("Found %d records for zone %s", len(records), zone.FQDN))

			endpoints = append(endpoints, mergeRecords(records, zone.FQDN)...)
		}
	}

	return endpoints, nil
}

// ApplyChanges applies a given set of changes.
func (p *Provider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	if p.dryRun {
		slog.Info("Dry run mode: changes would be applied but not actually executed")
		return p.printChanges(changes)
	}

	slog.Info("Requesting apply changes", "create", len(changes.Create), "update_old", len(changes.UpdateOld), "update_new", len(changes.UpdateNew), "delete", len(changes.Delete))

	// Process deletions first
	for _, ep := range changes.Delete {
		if err := p.deleteRecord(ctx, ep); err != nil {
			return err
		}
	}

	// Process creations
	for _, ep := range changes.Create {
		if err := p.createRecord(ctx, ep); err != nil {
			return err
		}
	}

	// Process updates
	for i, oldEp := range changes.UpdateOld {
		if i < len(changes.UpdateNew) {
			if err := p.updateRecord(ctx, oldEp, changes.UpdateNew[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Provider) GetDomainFilter() endpoint.DomainFilterInterface {
	return p.domainFilter
}

// AdjustEndpoints normalizes endpoints before ExternalDNS computes the diff.
func (p *Provider) AdjustEndpoints(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	for _, ep := range endpoints {
		// Infomaniak enforces a minimum TTL of 60 seconds, so any lower value is raised here.
		if ep.RecordTTL < minTTL {
			slog.Warn(fmt.Sprintf("TTL %d for %s is below Infomaniak minimum (%d), raising to %d", ep.RecordTTL, ep.DNSName, minTTL, minTTL))
			ep.RecordTTL = minTTL
		}
	}
	return endpoints, nil
}

// mergeRecords converts Infomaniak records into ExternalDNS endpoints, collapsing
// rows that share a name and type into one multi-target endpoint. Infomaniak keeps
// a separate row per target, so without this multi-target records (round-robin A,
// multiple MX) would look out of date and churn on every reconcile.
func mergeRecords(records []InfomaniakRecord, zoneFQDN string) []*endpoint.Endpoint {
	byKey := make(map[string]*endpoint.Endpoint)
	var order []string

	for _, record := range records {
		key := ensureFQDN(record.Source, zoneFQDN) + "/" + record.Type
		// The first row for a name/type builds the endpoint; later rows just add
		// their target to it.
		if ep, ok := byKey[key]; ok {
			ep.Targets = append(ep.Targets, normalizeReadTarget(record.Type, record.Target))
		} else {
			byKey[key] = recordToEndpoint(record, zoneFQDN)
			order = append(order, key)
		}
	}

	endpoints := make([]*endpoint.Endpoint, 0, len(order))
	for _, key := range order {
		endpoints = append(endpoints, byKey[key])
	}

	return endpoints
}

// recordToEndpoint converts a single Infomaniak record to an ExternalDNS endpoint.
func recordToEndpoint(r InfomaniakRecord, zoneFQDN string) *endpoint.Endpoint {
	dnsName := ensureFQDN(r.Source, zoneFQDN)
	// Normalize TTL to match what we send to the API
	ttl := max(r.TTL, minTTL)
	target := normalizeReadTarget(r.Type, r.Target)

	return endpoint.NewEndpointWithTTL(dnsName, r.Type, endpoint.TTL(ttl), target)
}

// normalizeReadTarget converts a target from Infomaniak's read form to the one
// ExternalDNS holds, so managed records don't look perpetually changed: SRV
// targets get their trailing dot (RFC 2782), and TXT values are unquoted with
// their 255-byte chunks joined.
func normalizeReadTarget(recordType, target string) string {
	switch recordType {
	case "SRV":
		// SRV target is "priority weight port host"; ensure the host ends with a
		// dot per RFC 2782. Infomaniak's API returns the four fields; a malformed
		// value is an upstream bug and should surface rather than be masked here.
		fields := strings.Fields(target)
		if !strings.HasSuffix(fields[3], ".") {
			fields[3] += "."

			return strings.Join(fields, " ")
		}
	case "TXT":
		return unquoteTXT(target)
	}

	return target
}

// unquoteTXT joins the one or more double-quoted character-strings Infomaniak
// returns for a TXT value into the raw value ExternalDNS holds (values over 255
// bytes come back split into several quoted chunks). Unquoted input is returned
// unchanged.
func unquoteTXT(target string) string {
	if !strings.HasPrefix(target, `"`) {
		return target
	}

	var b strings.Builder
	inQuotes := false
	escaped := false
	for _, r := range target {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\' && inQuotes:
			escaped = true
		case r == '"':
			inQuotes = !inQuotes
		case inQuotes:
			b.WriteRune(r)
		}
	}

	return b.String()
}

// ensureFQDN ensures the record name is a fully qualified domain name.
func ensureFQDN(source, zoneFQDN string) string {
	// Infomaniak uses "." for root records
	if source == "." {
		return zoneFQDN
	}

	return fmt.Sprintf("%s.%s", source, zoneFQDN)
}

// extractRecordSource extracts the record source (subdomain) from a full DNS name
// Returns "." for root records as expected by Infomaniak API
func extractRecordSource(dnsName, zoneFQDN string) string {
	if dnsName == zoneFQDN {
		return "."
	}

	if strings.HasSuffix(dnsName, "."+zoneFQDN) {
		return strings.TrimSuffix(dnsName, "."+zoneFQDN)
	}

	return dnsName
}

// findZoneForEndpoint finds the zone FQDN that matches the given endpoint
func (p *Provider) findZoneForEndpoint(ctx context.Context, ep *endpoint.Endpoint) (string, error) {
	domains, err := p.client.GetDomains(ctx)
	if err != nil {
		return "", err
	}

	var bestMatch string
	for _, domain := range domains {
		zones, err := p.client.GetDomainZones(ctx, domain.Name)
		if err != nil {
			slog.Warn(err.Error())
			continue
		}

		for _, zone := range zones {
			if ep.DNSName == zone.FQDN || strings.HasSuffix(ep.DNSName, "."+zone.FQDN) {
				if len(zone.FQDN) > len(bestMatch) {
					bestMatch = zone.FQDN
				}
			}
		}
	}

	if bestMatch == "" {
		return "", fmt.Errorf("no matching zone found for endpoint %s", ep.DNSName)
	}

	return bestMatch, nil
}

// findRecords returns the live Infomaniak rows for a given source and record type
// in a zone. Infomaniak keeps one row per target, so this may return several.
func (p *Provider) findRecords(ctx context.Context, zoneFQDN, source, recordType string) ([]InfomaniakRecord, error) {
	records, err := p.client.GetRecords(ctx, zoneFQDN)
	if err != nil {
		return nil, err
	}

	var matches []InfomaniakRecord
	for _, record := range records {
		if record.Source == source && record.Type == recordType {
			matches = append(matches, record)
		}
	}

	return matches, nil
}

// createRecord creates a new DNS record.
func (p *Provider) createRecord(ctx context.Context, ep *endpoint.Endpoint) error {
	zoneFQDN, err := p.findZoneForEndpoint(ctx, ep)
	if err != nil {
		return fmt.Errorf("failed to find zone: %w", err)
	}

	source := extractRecordSource(ep.DNSName, zoneFQDN)

	for _, target := range ep.Targets {
		record := RecordRequest{
			Source: source,
			Type:   ep.RecordType,
			Target: target,
			TTL:    max(int(ep.RecordTTL), minTTL),
		}

		_, err := p.client.CreateRecord(ctx, zoneFQDN, record)
		if err != nil {
			return err
		}

		slog.Info("Created record", "source", source, "record_type", ep.RecordType, "target", target)
	}

	return nil
}

// updateRecord applies a change as the delta between the old and new target sets:
// Infomaniak keeps one row per target, so rows for added targets are created and
// rows for removed targets deleted, leaving shared ones untouched. Deletions are
// derived from oldEp — the set ExternalDNS previously owned — never from the live
// records, so entries another tool manages in the same zone are left alone (they
// are never in oldEp). Live records are read only to resolve a target to its row ID
// and to avoid recreating one that already exists.
func (p *Provider) updateRecord(ctx context.Context, oldEp, newEp *endpoint.Endpoint) error {
	zoneFQDN, err := p.findZoneForEndpoint(ctx, oldEp)
	if err != nil {
		return fmt.Errorf("failed to find zone: %w", err)
	}

	source := extractRecordSource(oldEp.DNSName, zoneFQDN)

	records, err := p.findRecords(ctx, zoneFQDN, source, newEp.RecordType)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %w", err)
	}

	// Index live rows by normalized target, matching ExternalDNS's representation,
	// so a target can be resolved to its row.
	existing := make(map[string]InfomaniakRecord)
	for _, record := range records {
		existing[normalizeReadTarget(record.Type, record.Target)] = record
	}

	desired := make(map[string]bool, len(newEp.Targets))
	for _, target := range newEp.Targets {
		desired[target] = true
	}

	// Create rows for desired targets that do not exist yet.
	for _, target := range newEp.Targets {
		if _, ok := existing[target]; ok {
			continue
		}

		record := RecordRequest{
			Source: source,
			Type:   newEp.RecordType,
			Target: target,
			TTL:    max(int(newEp.RecordTTL), minTTL),
		}

		if _, err := p.client.CreateRecord(ctx, zoneFQDN, record); err != nil {
			return err
		}

		slog.Info("Updated record (added target)", "source", source, "record_type", newEp.RecordType, "target", target)
	}

	// Delete rows for targets ExternalDNS previously owned (oldEp) that are no longer
	// desired. Sourcing deletions from oldEp keeps us within ExternalDNS's ownership.
	for _, target := range oldEp.Targets {
		if desired[target] {
			continue
		}

		record, ok := existing[target]
		if !ok {
			continue
		}

		if err := p.client.DeleteRecord(ctx, zoneFQDN, record.ID); err != nil {
			return err
		}

		slog.Info("Updated record (removed target)", "source", source, "record_type", newEp.RecordType, "target", target)
	}

	return nil
}

// deleteRecord deletes a DNS record. Every Infomaniak row for the endpoint's
// (source, type) is removed, so multi-target records are deleted in full.
func (p *Provider) deleteRecord(ctx context.Context, ep *endpoint.Endpoint) error {
	zoneFQDN, err := p.findZoneForEndpoint(ctx, ep)
	if err != nil {
		return fmt.Errorf("failed to find zone: %w", err)
	}

	source := extractRecordSource(ep.DNSName, zoneFQDN)

	records, err := p.findRecords(ctx, zoneFQDN, source, ep.RecordType)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %w", err)
	}

	deleted := false
	for _, record := range records {
		if err := p.client.DeleteRecord(ctx, zoneFQDN, record.ID); err != nil {
			return err
		}
		deleted = true

		slog.Info("Deleted record", "source", source, "record_type", ep.RecordType, "target", record.Target)
	}

	if !deleted {
		// Record already doesn't exist, consider this a success.
		slog.Warn("Record not found for deletion", "source", source, "record_type", ep.RecordType)
	}

	return nil
}

// printChanges prints the changes that would be made in dry-run mode.
func (p *Provider) printChanges(changes *plan.Changes) error {
	if len(changes.Delete) > 0 {
		slog.Info("Would delete the following records:")
		for _, ep := range changes.Delete {
			slog.Info(fmt.Sprintf("  - %s %s", ep.DNSName, ep.RecordType))
		}
	}

	if len(changes.Create) > 0 {
		slog.Info("Would create the following records:")
		for _, ep := range changes.Create {
			slog.Info(fmt.Sprintf("  + %s %s %s", ep.DNSName, ep.RecordType, ep.Targets))
		}
	}

	if len(changes.UpdateOld) > 0 {
		slog.Info("Would update the following records:")
		for i, oldEp := range changes.UpdateOld {
			if i < len(changes.UpdateNew) {
				newEp := changes.UpdateNew[i]
				slog.Info(fmt.Sprintf("  ~ %s %s: %s %s", oldEp.DNSName, oldEp.RecordType, oldEp.Targets, newEp.Targets))
			}
		}
	}

	return nil
}
