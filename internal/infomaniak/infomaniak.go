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
		return nil, fmt.Errorf("failed to get domains: %w", err)
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
			slog.Warn(fmt.Sprintf("Failed to get zones for domain %s: %v", domain.Name, err))
			continue
		}

		for _, zone := range zones {
			// Get records for this zone
			records, err := p.client.GetRecords(ctx, zone.FQDN)
			if err != nil {
				slog.Warn(fmt.Sprintf("Failed to get records for zone %s: %v", zone.FQDN, err))
				continue
			}

			slog.Debug(fmt.Sprintf("Found %d records for zone %s", len(records), zone.FQDN))

			for _, record := range records {
				// Convert record to endpoint
				if ep := recordToEndpoint(record, zone.FQDN); ep != nil {
					endpoints = append(endpoints, ep)
				}
			}
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

	slog.Info(fmt.Sprintf("requesting apply changes, create: %d, updateOld: %d, updateNew: %d, delete: %d", len(changes.Create), len(changes.UpdateOld), len(changes.UpdateNew), len(changes.Delete)))

	// Process deletions first
	for _, ep := range changes.Delete {
		if err := p.deleteRecord(ctx, ep); err != nil {
			return fmt.Errorf("failed to delete record %s: %w", ep.DNSName, err)
		}
	}

	// Process creations
	for _, ep := range changes.Create {
		if err := p.createRecord(ctx, ep); err != nil {
			return fmt.Errorf("failed to create record %s: %w", ep.DNSName, err)
		}
	}

	// Process updates
	for i, oldEp := range changes.UpdateOld {
		if i < len(changes.UpdateNew) {
			if err := p.updateRecord(ctx, oldEp, changes.UpdateNew[i]); err != nil {
				return fmt.Errorf("failed to update record %s: %w", oldEp.DNSName, err)
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

// recordToEndpoint converts an Infomaniak record to an ExternalDNS endpoint.
func recordToEndpoint(r InfomaniakRecord, zoneFQDN string) *endpoint.Endpoint {
	dnsName := ensureFQDN(r.Source, zoneFQDN)
	// Normalize TTL to match what we send to the API
	ttl := max(r.TTL, minTTL)
	return endpoint.NewEndpointWithTTL(dnsName, r.Type, endpoint.TTL(ttl), r.Target)
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
		return "", fmt.Errorf("failed to get domains: %w", err)
	}

	var bestMatch string
	for _, domain := range domains {
		zones, err := p.client.GetDomainZones(ctx, domain.Name)
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to get zones for domain %s: %v", domain.Name, err))
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

// findRecord finds an existing record matching the endpoint
func (p *Provider) findRecord(ctx context.Context, zoneFQDN string, source, recordType string) (*InfomaniakRecord, error) {
	records, err := p.client.GetRecords(ctx, zoneFQDN)
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		if record.Source == source && record.Type == recordType {
			return &record, nil
		}
	}

	return nil, nil
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

		// Set priority for MX and SRV records
		if ep.RecordType == "MX" || ep.RecordType == "SRV" {
			record.Priority = 10
		}

		_, err := p.client.CreateRecord(ctx, zoneFQDN, record)
		if err != nil {
			return fmt.Errorf("failed to create record: %w", err)
		}

		slog.Info(fmt.Sprintf("Created record %s %s %s", source, ep.RecordType, target))
	}

	return nil
}

// updateRecord updates an existing DNS record.
func (p *Provider) updateRecord(ctx context.Context, oldEp, newEp *endpoint.Endpoint) error {
	zoneFQDN, err := p.findZoneForEndpoint(ctx, oldEp)
	if err != nil {
		return fmt.Errorf("failed to find zone: %w", err)
	}

	source := extractRecordSource(oldEp.DNSName, zoneFQDN)

	// Find the existing record
	existingRecord, err := p.findRecord(ctx, zoneFQDN, source, oldEp.RecordType)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %w", err)
	}

	if existingRecord == nil {
		return fmt.Errorf("record not found for update: %s %s", source, oldEp.RecordType)
	}

	// Update with new values
	record := RecordRequest{
		Source: source,
		Type:   newEp.RecordType,
		Target: newEp.Targets[0],
		TTL:    max(int(newEp.RecordTTL), minTTL),
	}

	if newEp.RecordType == "MX" || newEp.RecordType == "SRV" {
		record.Priority = existingRecord.Priority
		if record.Priority == 0 {
			record.Priority = 10
		}
	}

	_, err = p.client.UpdateRecord(ctx, zoneFQDN, existingRecord.ID, record)
	if err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}

	slog.Info(fmt.Sprintf("Updated record %s %s %s", source, newEp.RecordType, newEp.Targets[0]))
	return nil
}

// deleteRecord deletes a DNS record.
func (p *Provider) deleteRecord(ctx context.Context, ep *endpoint.Endpoint) error {
	zoneFQDN, err := p.findZoneForEndpoint(ctx, ep)
	if err != nil {
		return fmt.Errorf("failed to find zone: %w", err)
	}

	source := extractRecordSource(ep.DNSName, zoneFQDN)

	// Find the existing record
	existingRecord, err := p.findRecord(ctx, zoneFQDN, source, ep.RecordType)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %w", err)
	}

	if existingRecord == nil {
		// Record already doesn't exist, consider this a success
		slog.Warn(fmt.Sprintf("Record not found for deletion: %s %s", source, ep.RecordType))
		return nil
	}

	err = p.client.DeleteRecord(ctx, zoneFQDN, existingRecord.ID)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	slog.Info(fmt.Sprintf("Deleted record %s %s", source, ep.RecordType))
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
