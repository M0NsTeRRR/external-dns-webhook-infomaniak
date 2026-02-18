package infomaniak

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInfomaniakClient_NewInfomaniakClient tests the client initialization
func TestInfomaniakClient_NewInfomaniakClient(t *testing.T) {
	config := &Config{
		APIToken: "test-token",
		Debug:    true,
		DryRun:   false,
	}

	client := NewInfomaniakClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.config)
	assert.NotNil(t, client.client)
}

// TestInfomaniakClient_GetAccounts tests the GetAccounts method
func TestInfomaniakClient_GetAccounts(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/1/account", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		response := AccountListResponse{
			Data: []InfomaniakAccount{
				{ID: 1, Name: "Test Account 1"},
				{ID: 2, Name: "Test Account 2"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with test configuration
	config := &Config{
		APIToken: "test-token",
		Debug:    false,
		DryRun:   false,
	}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL

	// Test the method
	accounts, err := client.GetAccounts(context.Background())

	require.NoError(t, err)
	assert.Len(t, accounts, 2)
	assert.Equal(t, 1, accounts[0].ID)
	assert.Equal(t, "Test Account 1", accounts[0].Name)
	assert.Equal(t, 2, accounts[1].ID)
	assert.Equal(t, "Test Account 2", accounts[1].Name)
}

// TestInfomaniakClient_GetZones tests the GetZones method
func TestInfomaniakClient_GetZones(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/1/domain/1/zone", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		response := ZoneListResponse{
			Data: []InfomaniakZone{
				{ID: 101, Name: "example.com", Status: "active"},
				{ID: 102, Name: "test.com", Status: "active"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with test configuration
	config := &Config{
		APIToken: "test-token",
		Debug:    false,
		DryRun:   false,
	}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL

	// Test the method
	zones, err := client.GetZones(context.Background(), 1)

	require.NoError(t, err)
	assert.Len(t, zones, 2)
	assert.Equal(t, 101, zones[0].ID)
	assert.Equal(t, "example.com", zones[0].Name)
	assert.Equal(t, 102, zones[1].ID)
	assert.Equal(t, "test.com", zones[1].Name)
}

// TestInfomaniakClient_GetRecords tests the GetRecords method
func TestInfomaniakClient_GetRecords(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/1/domain/zone/101/record", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		response := RecordListResponse{
			Data: []InfomaniakRecord{
				{
					ID:      1001,
					ZoneID:  101,
					Name:    "@",
					Type:    "A",
					Content: "192.0.2.1",
					TTL:     3600,
					Pri:     0,
				},
				{
					ID:      1002,
					ZoneID:  101,
					Name:    "www",
					Type:    "CNAME",
					Content: "example.com",
					TTL:     1800,
					Pri:     0,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with test configuration
	config := &Config{
		APIToken: "test-token",
		Debug:    false,
		DryRun:   false,
	}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL

	// Test the method
	records, err := client.GetRecords(context.Background(), 101)

	require.NoError(t, err)
	assert.Len(t, records, 2)
	assert.Equal(t, 1001, records[0].ID)
	assert.Equal(t, "@", records[0].Name)
	assert.Equal(t, "A", records[0].Type)
	assert.Equal(t, "192.0.2.1", records[0].Content)
	assert.Equal(t, 1002, records[1].ID)
	assert.Equal(t, "www", records[1].Name)
	assert.Equal(t, "CNAME", records[1].Type)
	assert.Equal(t, "example.com", records[1].Content)
}

// TestInfomaniakClient_doRequest tests the doRequest method
func TestInfomaniakClient_doRequest(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data": "test", "error": ""}`)
		}))
		defer server.Close()

		config := &Config{
			APIToken: "test-token",
			Debug:    false,
			DryRun:   false,
		}
		client := NewInfomaniakClient(config)
		client.client = server.Client()
		client.baseURL = server.URL

		body, err := client.doRequest(context.Background(), "GET", "/test", nil)

		require.NoError(t, err)
		assert.Contains(t, string(body), "test")
	})

	t.Run("failed request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error": "internal server error"}`)
		}))
		defer server.Close()

		config := &Config{
			APIToken: "test-token",
			Debug:    false,
			DryRun:   false,
		}
		client := NewInfomaniakClient(config)
		client.client = server.Client()
		client.baseURL = server.URL

		_, err := client.doRequest(context.Background(), "GET", "/test", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API request failed with status 500")
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate a slow response
			select {
			case <-r.Context().Done():
				w.WriteHeader(http.StatusRequestTimeout)
			case <-time.After(1 * time.Second):
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"data": "test"}`)
			}
		}))
		defer server.Close()

		config := &Config{
			APIToken: "test-token",
			Debug:    false,
			DryRun:   false,
		}
		client := NewInfomaniakClient(config)
		client.client = server.Client()
		client.baseURL = server.URL

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err := client.doRequest(ctx, "GET", "/test", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

// TestInfomaniakClient_ErrorHandling tests error handling
func TestInfomaniakClient_ErrorHandling(t *testing.T) {
	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data": null, "error": "authentication failed"}`)
		}))
		defer server.Close()

		config := &Config{
			APIToken: "test-token",
			Debug:    false,
			DryRun:   false,
		}
		client := NewInfomaniakClient(config)
		client.client = server.Client()
		client.baseURL = server.URL

		_, err := client.GetAccounts(context.Background())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API error: authentication failed")
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data": [, "error": ""}`) // Invalid JSON
		}))
		defer server.Close()

		config := &Config{
			APIToken: "test-token",
			Debug:    false,
			DryRun:   false,
		}
		client := NewInfomaniakClient(config)
		client.client = server.Client()
		client.baseURL = server.URL

		_, err := client.GetAccounts(context.Background())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal accounts response")
	})
}