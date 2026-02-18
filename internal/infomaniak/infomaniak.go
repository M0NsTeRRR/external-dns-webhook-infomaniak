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

// Provider implements the DNS provider for Infomaniak DNS.
type Provider struct {
	provider.BaseProvider
	client       *InfomaniakClient
	dryRun       bool
	domainFilter endpoint.DomainFilterInterface
}

func NewInfomaniakProvider(domanfilter endpoint.DomainFilterInterface, configuration *Config) *Provider {
	return &Provider{
		client:       NewInfomaniakClient(configuration),
		dryRun:       configuration.DryRun,
		domainFilter: domanfilter,
	}
}

// Records returns the list of resource records in all zones.
func (p *Provider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	var endpoints []*endpoint.Endpoint

	// Get all accounts
	accounts, err := p.client.GetAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	// For each account, get zones and records
	for _, account := range accounts {
		zones, err := p.client.GetZones(ctx, account.ID)
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to get zones for account %d: %v", account.ID, err))
			continue
		}

		for _, zone := range zones {
			// Apply domain filter if specified
			if p.domainFilter != nil && !p.domainFilter.Match(zone.Name) {
				slog.Debug(fmt.Sprintf("Skipping zone %s due to domain filter", zone.Name))
				continue
			}

			records, err := p.client.GetRecords(ctx, zone.ID)
			if err != nil {
				slog.Warn(fmt.Sprintf("Failed to get records for zone %d (%s): %v", zone.ID, zone.Name, err))
				continue
			}

			for _, record := range records {
				// Skip root records (@) and convert to proper endpoint format
				if record.Name == "@" {
					record.Name = zone.Name
				} else if !strings.HasSuffix(record.Name, "."+zone.Name) {
					record.Name = fmt.Sprintf("%s.%s", record.Name, zone.Name)
				}

				// Convert record to endpoint
				if ep := recordToEndpoint(record, zone.Name); ep != nil {
					endpoints = append(endpoints, ep)
				}
			}
		}
	}

	slog.Debug(fmt.Sprintf("Records() found %d endpoints", len(endpoints)))
	return endpoints, nil
}

// ApplyChanges applies a given set of changes.
func (p *Provider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	if p.dryRun {
		slog.Info("Dry run mode: changes would be applied but not actually executed")
		return p.printChanges(changes)
	}

	// Process deletions first
	for _, endpoint := range changes.Delete {
		if err := p.deleteRecord(ctx, endpoint); err != nil {
			return fmt.Errorf("failed to delete record %s: %w", endpoint.DNSName, err)
		}
	}

	// Process creations
	for _, endpoint := range changes.Create {
		if err := p.createRecord(ctx, endpoint); err != nil {
			return fmt.Errorf("failed to create record %s: %w", endpoint.DNSName, err)
		}
	}

	// Process updates
	for i, endpoint := range changes.UpdateOld {
		if i < len(changes.UpdateNew) {
			if err := p.updateRecord(ctx, endpoint, changes.UpdateNew[i]); err != nil {
				return fmt.Errorf("failed to update record %s: %w", endpoint.DNSName, err)
			}
		}
	}

	return nil
}

func (p *Provider) GetDomainFilter() endpoint.DomainFilterInterface {
	return p.domainFilter
}

// recordToEndpoint converts an Infomaniak record to an ExternalDNS endpoint.
func recordToEndpoint(r InfomaniakRecord, zoneName string) *endpoint.Endpoint {
	return endpoint.NewEndpointWithTTL(ensureFQDN(r.Name, zoneName), r.Type, endpoint.TTL(r.TTL), r.Content)
}

// ensureFQDN ensures the record name is a fully qualified domain name.
func ensureFQDN(name, zoneName string) string {
	if name == "@" || name == zoneName {
		return zoneName
	}

	if strings.HasSuffix(name, "."+zoneName) {
		return name
	}

	return fmt.Sprintf("%s.%s", name, zoneName)
}

// extractZoneIDAndRecordName extracts the zone ID and record name from an endpoint.
func (p *Provider) extractZoneIDAndRecordName(ctx context.Context, ep *endpoint.Endpoint) (int, string, error) {
	// Get all zones to find the matching one
	accounts, err := p.client.GetAccounts(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get accounts: %w", err)
	}

	for _, account := range accounts {
		zones, err := p.client.GetZones(ctx, account.ID)
		if err != nil {
			continue
		}

		for _, zone := range zones {
			if strings.HasSuffix(ep.DNSName, "."+zone.Name) || ep.DNSName == zone.Name {
				// Found the matching zone
				recordName := ep.DNSName
				if recordName == zone.Name {
					recordName = "@"
				} else if strings.HasSuffix(recordName, "."+zone.Name) {
					recordName = strings.TrimSuffix(recordName, "."+zone.Name)
				}
				return zone.ID, recordName, nil
			}
		}
	}

	return 0, "", fmt.Errorf("no matching zone found for endpoint %s", ep.DNSName)
}

// createRecord creates a new DNS record.
func (p *Provider) createRecord(ctx context.Context, ep *endpoint.Endpoint) error {
	zoneID, recordName, err := p.extractZoneIDAndRecordName(ctx, ep)
	if err != nil {
		return fmt.Errorf("failed to extract zone info: %w", err)
	}

	// Prepare the record data for Infomaniak API
	recordData := map[string]interface{}{
		"name":    recordName,
		"type":    ep.RecordType,
		"content": ep.Targets[0],
		"ttl":     int(ep.RecordTTL),
	}

	if ep.RecordType == "MX" || ep.RecordType == "SRV" {
		// For MX and SRV records, we might need priority
		// This would need to be extracted from the target format
		recordData["pri"] = 10
	}

	endpoint := fmt.Sprintf("/1/domain/zone/%d/record", zoneID)
	_, err = p.client.doRequest(ctx, "POST", endpoint, recordData)
	if err != nil {
		return fmt.Errorf("failed to create record: %w", err)
	}

	slog.Info(fmt.Sprintf("Created record %s %s -> %s", recordName, ep.RecordType, ep.Targets[0]))
	return nil
}

// updateRecord updates an existing DNS record.
func (p *Provider) updateRecord(ctx context.Context, oldEp, newEp *endpoint.Endpoint) error {
	zoneID, recordName, err := p.extractZoneIDAndRecordName(ctx, oldEp)
	if err != nil {
		return fmt.Errorf("failed to extract zone info: %w", err)
	}

	// First, we need to find the record ID
	existingRecords, err := p.client.GetRecords(ctx, zoneID)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %w", err)
	}

	var recordID int
	for _, record := range existingRecords {
		if record.Name == recordName && record.Type == oldEp.RecordType {
			recordID = record.ID
			break
		}
	}

	if recordID == 0 {
		return fmt.Errorf("record not found for update: %s %s", recordName, oldEp.RecordType)
	}

	// Prepare the updated record data
	updateData := map[string]interface{}{
		"content": newEp.Targets[0],
		"ttl":     int(newEp.RecordTTL),
	}

	endpoint := fmt.Sprintf("/1/domain/zone/%d/record/%d", zoneID, recordID)
	_, err = p.client.doRequest(ctx, "PUT", endpoint, updateData)
	if err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}

	slog.Info(fmt.Sprintf("Updated record %s %s -> %s", recordName, newEp.RecordType, newEp.Targets[0]))
	return nil
}

// deleteRecord deletes a DNS record.
func (p *Provider) deleteRecord(ctx context.Context, ep *endpoint.Endpoint) error {
	zoneID, recordName, err := p.extractZoneIDAndRecordName(ctx, ep)
	if err != nil {
		return fmt.Errorf("failed to extract zone info: %w", err)
	}

	// First, we need to find the record ID
	existingRecords, err := p.client.GetRecords(ctx, zoneID)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %w", err)
	}

	var recordID int
	for _, record := range existingRecords {
		if record.Name == recordName && record.Type == ep.RecordType {
			recordID = record.ID
			break
		}
	}

	if recordID == 0 {
		// Record already doesn't exist, consider this a success
		slog.Warn(fmt.Sprintf("Record not found for deletion: %s %s", recordName, ep.RecordType))
		return nil
	}

	endpoint := fmt.Sprintf("/1/domain/zone/%d/record/%d", zoneID, recordID)
	_, err = p.client.doRequest(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	slog.Info(fmt.Sprintf("Deleted record %s %s", recordName, ep.RecordType))
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
			slog.Info(fmt.Sprintf("  + %s %s -> %s", ep.DNSName, ep.RecordType, ep.Targets))
		}
	}

	if len(changes.UpdateOld) > 0 {
		slog.Info("Would update the following records:")
		for i, oldEp := range changes.UpdateOld {
			if i < len(changes.UpdateNew) {
				newEp := changes.UpdateNew[i]
				slog.Info(fmt.Sprintf("  ~ %s %s: %s -> %s", oldEp.DNSName, oldEp.RecordType, oldEp.Targets, newEp.Targets))
			}
		}
	}

	return nil
}
