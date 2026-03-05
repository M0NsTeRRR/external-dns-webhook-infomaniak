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

func TestInfomaniakClient_NewInfomaniakClient(t *testing.T) {
	config := &Config{
		APIToken: "test-token",
		DryRun:   false,
	}

	client := NewInfomaniakClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.config)
	assert.NotNil(t, client.client)
	assert.Equal(t, "https://api.infomaniak.com", client.baseURL)
}

func TestInfomaniakClient_GetDomains(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/2/domains/domains", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		response := DomainListResponse{
			Result: "success",
			Data: []InfomaniakDomain{
				{Name: "example.com"},
				{Name: "test.com"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL

	domains, err := client.GetDomains(context.Background())

	require.NoError(t, err)
	assert.Len(t, domains, 2)
	assert.Equal(t, "example.com", domains[0].Name)
	assert.Equal(t, "test.com", domains[1].Name)
}

func TestInfomaniakClient_GetDomainZones(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/2/domains/domains/example.com/zones", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		response := ZoneListResponse{
			Result: "success",
			Data: []InfomaniakZone{
				{FQDN: "example.com"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL

	zones, err := client.GetDomainZones(context.Background(), "example.com")

	require.NoError(t, err)
	assert.Len(t, zones, 1)
	assert.Equal(t, "example.com", zones[0].FQDN)
}

func TestInfomaniakClient_GetRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/2/zones/example.com/records", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		response := RecordListResponse{
			Result: "success",
			Data: []InfomaniakRecord{
				{ID: 1001, Source: "@", Type: "A", Target: "192.0.2.1", TTL: 3600},
				{ID: 1002, Source: "www", Type: "CNAME", Target: "example.com", TTL: 1800},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL

	records, err := client.GetRecords(context.Background(), "example.com")

	require.NoError(t, err)
	assert.Len(t, records, 2)
	assert.Equal(t, 1001, records[0].ID)
	assert.Equal(t, "@", records[0].Source)
	assert.Equal(t, "A", records[0].Type)
	assert.Equal(t, "192.0.2.1", records[0].Target)
	assert.Equal(t, 1002, records[1].ID)
	assert.Equal(t, "www", records[1].Source)
	assert.Equal(t, "CNAME", records[1].Type)
}

func TestInfomaniakClient_CreateRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/2/zones/example.com/records", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req RecordRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "test", req.Source)
		assert.Equal(t, "A", req.Type)
		assert.Equal(t, "192.0.2.10", req.Target)
		assert.Equal(t, 3600, req.TTL)

		response := RecordCreateResponse{
			Result: "success",
			Data: InfomaniakRecord{
				ID:     1003,
				Source: req.Source,
				Type:   req.Type,
				Target: req.Target,
				TTL:    req.TTL,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL

	record, err := client.CreateRecord(context.Background(), "example.com", RecordRequest{
		Source: "test",
		Type:   "A",
		Target: "192.0.2.10",
		TTL:    3600,
	})

	require.NoError(t, err)
	assert.Equal(t, 1003, record.ID)
	assert.Equal(t, "test", record.Source)
	assert.Equal(t, "A", record.Type)
	assert.Equal(t, "192.0.2.10", record.Target)
}

func TestInfomaniakClient_UpdateRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/2/zones/example.com/records/1001", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var req RecordRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "test", req.Source)
		assert.Equal(t, "A", req.Type)
		assert.Equal(t, "192.0.2.20", req.Target)

		response := RecordCreateResponse{
			Result: "success",
			Data: InfomaniakRecord{
				ID:     1001,
				Source: req.Source,
				Type:   req.Type,
				Target: req.Target,
				TTL:    req.TTL,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL

	record, err := client.UpdateRecord(context.Background(), "example.com", 1001, RecordRequest{
		Source: "test",
		Type:   "A",
		Target: "192.0.2.20",
		TTL:    3600,
	})

	require.NoError(t, err)
	assert.Equal(t, 1001, record.ID)
	assert.Equal(t, "192.0.2.20", record.Target)
}

func TestInfomaniakClient_DeleteRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/2/zones/example.com/records/1001", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		response := APIResponse{Result: "success", Data: nil}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL

	err := client.DeleteRecord(context.Background(), "example.com", 1001)

	require.NoError(t, err)
}

func TestInfomaniakClient_doRequest(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"result": "success", "data": "test"}`)
		}))
		defer server.Close()

		config := &Config{APIToken: "test-token", DryRun: false}
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

		config := &Config{APIToken: "test-token", DryRun: false}
		client := NewInfomaniakClient(config)
		client.client = server.Client()
		client.baseURL = server.URL

		_, err := client.doRequest(context.Background(), "GET", "/test", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API request failed with status 500")
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
				w.WriteHeader(http.StatusRequestTimeout)
			case <-time.After(1 * time.Second):
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"result": "success", "data": "test"}`)
			}
		}))
		defer server.Close()

		config := &Config{APIToken: "test-token", DryRun: false}
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

func TestInfomaniakClient_ErrorHandling(t *testing.T) {
	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"result": "error", "data": null, "error": "authentication failed"}`)
		}))
		defer server.Close()

		config := &Config{APIToken: "test-token", DryRun: false}
		client := NewInfomaniakClient(config)
		client.client = server.Client()
		client.baseURL = server.URL

		_, err := client.GetRecords(context.Background(), "example.com")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API error")
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data": [, "error": ""}`)
		}))
		defer server.Close()

		config := &Config{APIToken: "test-token", DryRun: false}
		client := NewInfomaniakClient(config)
		client.client = server.Client()
		client.baseURL = server.URL

		_, err := client.GetRecords(context.Background(), "example.com")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal records response")
	})
}
