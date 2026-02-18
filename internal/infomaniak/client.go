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

	if c.config.Debug {
		slog.Debug(fmt.Sprintf("Request: %s %s", method, url))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// GetAccounts retrieves the list of Infomaniak accounts with context support
func (c *InfomaniakClient) GetAccounts(ctx context.Context) ([]InfomaniakAccount, error) {
	body, err := c.doRequest(ctx, "GET", "/1/account", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	var response AccountListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal accounts response: %w", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("API error: %s", response.Error)
	}

	return response.Data, nil
}

// GetZones retrieves the list of DNS zones for a given account with context support
func (c *InfomaniakClient) GetZones(ctx context.Context, accountID int) ([]InfomaniakZone, error) {
	endpoint := fmt.Sprintf("/1/domain/%d/zone", accountID)
	body, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get zones for account %d: %w", accountID, err)
	}

	var response ZoneListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal zones response: %w", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("API error: %s", response.Error)
	}

	return response.Data, nil
}

// GetRecords retrieves the list of DNS records for a given zone with context support
func (c *InfomaniakClient) GetRecords(ctx context.Context, zoneID int) ([]InfomaniakRecord, error) {
	endpoint := fmt.Sprintf("/1/domain/zone/%d/record", zoneID)
	body, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get records for zone %d: %w", zoneID, err)
	}

	var response RecordListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal records response: %w", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("API error: %s", response.Error)
	}

	return response.Data, nil
}
