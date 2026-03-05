package infomaniak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type InfomaniakClient struct {
	config  *Config
	client  *http.Client
	baseURL string
}

func NewInfomaniakClient(config *Config) *InfomaniakClient {
	slog.Info("creating a new client for Infomaniak API Client")
	return &InfomaniakClient{
		config:  config,
		client:  &http.Client{},
		baseURL: "https://api.infomaniak.com",
	}
}

// doRequest performs an HTTP request to the Infomaniak API with context support
func (c *InfomaniakClient) doRequest(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	url := c.baseURL + endpoint

	var req *http.Request
	var err error

	if body != nil {
		var jsonBody []byte
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(jsonBody))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.APIToken)
	req.Header.Set("Content-Type", "application/json")

	slog.Debug(fmt.Sprintf("Request: %s %s", method, url))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}

// GetDomains retrieves the list of domains using v2 API
// GET /2/domains/domains
func (c *InfomaniakClient) GetDomains(ctx context.Context) ([]InfomaniakDomain, error) {
	body, err := c.doRequest(ctx, "GET", "/2/domains/domains", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get domains: %w", err)
	}

	var response DomainListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal domains response: %w", err)
	}

	if response.Result != "success" {
		return nil, fmt.Errorf("API error: %v", response.Error)
	}

	return response.Data, nil
}

// GetDomainZones retrieves the list of DNS zones for a given domain using v2 API
// GET /2/domains/domains/{domain}/zones
func (c *InfomaniakClient) GetDomainZones(ctx context.Context, domainName string) ([]InfomaniakZone, error) {
	endpoint := fmt.Sprintf("/2/domains/domains/%s/zones", domainName)
	body, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get zones for domain %s: %w", domainName, err)
	}

	var response ZoneListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal zones response: %w", err)
	}

	if response.Result != "success" {
		return nil, fmt.Errorf("API error: %v", response.Error)
	}

	return response.Data, nil
}

// GetRecords retrieves the list of DNS records for a given zone using v2 API
// GET /2/zones/{zone}/records
func (c *InfomaniakClient) GetRecords(ctx context.Context, zoneFQDN string) ([]InfomaniakRecord, error) {
	endpoint := fmt.Sprintf("/2/zones/%s/records", zoneFQDN)
	body, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get records for zone %s: %w", zoneFQDN, err)
	}

	var response RecordListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal records response: %w", err)
	}

	if response.Result != "success" {
		return nil, fmt.Errorf("API error: %v", response.Error)
	}

	return response.Data, nil
}

// CreateRecord creates a new DNS record in a zone
// POST /2/zones/{zone}/records
func (c *InfomaniakClient) CreateRecord(ctx context.Context, zoneFQDN string, record RecordRequest) (*InfomaniakRecord, error) {
	endpoint := fmt.Sprintf("/2/zones/%s/records", zoneFQDN)
	body, err := c.doRequest(ctx, "POST", endpoint, record)
	if err != nil {
		return nil, fmt.Errorf("failed to create record: %w", err)
	}

	var response RecordCreateResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal create response: %w", err)
	}

	if response.Result != "success" {
		return nil, fmt.Errorf("API error: %v", response.Error)
	}

	return &response.Data, nil
}

// UpdateRecord updates an existing DNS record
// PUT /2/zones/{zone}/records/{record}
func (c *InfomaniakClient) UpdateRecord(ctx context.Context, zoneFQDN string, recordID int, record RecordRequest) (*InfomaniakRecord, error) {
	endpoint := fmt.Sprintf("/2/zones/%s/records/%d", zoneFQDN, recordID)
	body, err := c.doRequest(ctx, "PUT", endpoint, record)
	if err != nil {
		return nil, fmt.Errorf("failed to update record: %w", err)
	}

	var response RecordCreateResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal update response: %w", err)
	}

	if response.Result != "success" {
		return nil, fmt.Errorf("API error: %v", response.Error)
	}

	return &response.Data, nil
}

// DeleteRecord deletes a DNS record
// DELETE /2/zones/{zone}/records/{record}
func (c *InfomaniakClient) DeleteRecord(ctx context.Context, zoneFQDN string, recordID int) error {
	endpoint := fmt.Sprintf("/2/zones/%s/records/%d", zoneFQDN, recordID)
	body, err := c.doRequest(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	var response APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to unmarshal delete response: %w", err)
	}

	if response.Result != "success" {
		return fmt.Errorf("API error: %v", response.Error)
	}

	return nil
}
