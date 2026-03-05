package infomaniak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

func TestNewInfomaniakProvider(t *testing.T) {
	config := &Config{
		APIToken: "test-token",
		DryRun:   false,
	}

	provider := NewInfomaniakProvider(nil, config)

	assert.NotNil(t, provider)
	assert.NotNil(t, provider.client)
	assert.Equal(t, config.DryRun, provider.dryRun)
	assert.Equal(t, config, provider.client.config)
}

func TestProvider_Records(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/2/domains/domains":
			response := DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "example.com"}},
			}
			json.NewEncoder(w).Encode(response)
		case "/2/domains/domains/example.com/zones":
			response := ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "example.com"}},
			}
			json.NewEncoder(w).Encode(response)
		case "/2/zones/example.com/records":
			response := RecordListResponse{
				Result: "success",
				Data: []InfomaniakRecord{
					{ID: 1, Source: ".", Type: "A", Target: "192.0.2.1", TTL: 3600},
					{ID: 2, Source: "www", Type: "CNAME", Target: "example.com", TTL: 1800},
				},
			}
			json.NewEncoder(w).Encode(response)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL

	provider := &Provider{client: client, dryRun: false, domainFilter: nil}

	endpoints, err := provider.Records(context.Background())

	require.NoError(t, err)
	assert.Len(t, endpoints, 2)
	assert.Equal(t, "example.com", endpoints[0].DNSName)
	assert.Equal(t, "A", endpoints[0].RecordType)
	assert.Equal(t, "www.example.com", endpoints[1].DNSName)
	assert.Equal(t, "CNAME", endpoints[1].RecordType)
}

func TestProvider_ApplyChanges_DryRun(t *testing.T) {
	config := &Config{APIToken: "test-token", DryRun: true}
	client := NewInfomaniakClient(config)

	provider := &Provider{client: client, dryRun: true, domainFilter: nil}

	changes := &plan.Changes{
		Delete: []*endpoint.Endpoint{endpoint.NewEndpoint("test.example.com", "A", "192.0.2.1")},
		Create: []*endpoint.Endpoint{endpoint.NewEndpoint("new.example.com", "A", "192.0.2.2")},
	}

	err := provider.ApplyChanges(context.Background(), changes)

	require.NoError(t, err)
}

func TestProvider_CreateRecord(t *testing.T) {
	createdRecord := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/2/domains/domains" && r.Method == "GET":
			response := DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "example.com"}},
			}
			json.NewEncoder(w).Encode(response)
		case r.URL.Path == "/2/domains/domains/example.com/zones" && r.Method == "GET":
			response := ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "example.com"}},
			}
			json.NewEncoder(w).Encode(response)
		case r.URL.Path == "/2/zones/example.com/records" && r.Method == "POST":
			createdRecord = true
			var req RecordRequest
			json.NewDecoder(r.Body).Decode(&req)
			response := RecordCreateResponse{
				Result: "success",
				Data: InfomaniakRecord{
					ID:     100,
					Source: req.Source,
					Type:   req.Type,
					Target: req.Target,
					TTL:    req.TTL,
				},
			}
			json.NewEncoder(w).Encode(response)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL

	provider := &Provider{client: client, dryRun: false, domainFilter: nil}

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{endpoint.NewEndpointWithTTL("test.example.com", "A", 3600, "192.0.2.10")},
	}

	err := provider.ApplyChanges(context.Background(), changes)

	require.NoError(t, err)
	assert.True(t, createdRecord, "Expected record to be created")
}

func TestProvider_DeleteRecord(t *testing.T) {
	deletedRecord := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/2/domains/domains" && r.Method == "GET":
			response := DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "example.com"}},
			}
			json.NewEncoder(w).Encode(response)
		case r.URL.Path == "/2/domains/domains/example.com/zones" && r.Method == "GET":
			response := ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "example.com"}},
			}
			json.NewEncoder(w).Encode(response)
		case r.URL.Path == "/2/zones/example.com/records" && r.Method == "GET":
			response := RecordListResponse{
				Result: "success",
				Data:   []InfomaniakRecord{{ID: 1, Source: "test", Type: "A", Target: "192.0.2.1", TTL: 3600}},
			}
			json.NewEncoder(w).Encode(response)
		case r.URL.Path == "/2/zones/example.com/records/1" && r.Method == "DELETE":
			deletedRecord = true
			response := APIResponse{Result: "success"}
			json.NewEncoder(w).Encode(response)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL

	provider := &Provider{client: client, dryRun: false, domainFilter: nil}

	changes := &plan.Changes{
		Delete: []*endpoint.Endpoint{endpoint.NewEndpoint("test.example.com", "A", "192.0.2.1")},
	}

	err := provider.ApplyChanges(context.Background(), changes)

	require.NoError(t, err)
	assert.True(t, deletedRecord, "Expected record to be deleted")
}

func TestProvider_AdjustEndpoints(t *testing.T) {
	provider := &Provider{}

	endpoints := []*endpoint.Endpoint{
		endpoint.NewEndpointWithTTL("a.example.com", "A", 30, "1.2.3.4"),  // below min → raised
		endpoint.NewEndpointWithTTL("b.example.com", "A", 60, "1.2.3.5"),  // equal min → unchanged
		endpoint.NewEndpointWithTTL("c.example.com", "A", 300, "1.2.3.6"), // above min → unchanged
		endpoint.NewEndpointWithTTL("d.example.com", "A", 0, "1.2.3.7"),   // zero → raised
	}

	result, err := provider.AdjustEndpoints(endpoints)
	require.NoError(t, err)
	assert.Equal(t, endpoint.TTL(minTTL), result[0].RecordTTL)
	assert.Equal(t, endpoint.TTL(minTTL), result[1].RecordTTL)
	assert.Equal(t, endpoint.TTL(300), result[2].RecordTTL)
	assert.Equal(t, endpoint.TTL(minTTL), result[3].RecordTTL)
}

func TestProvider_MostSpecificZone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/2/domains/domains" && r.Method == "GET":
			json.NewEncoder(w).Encode(DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "test.fr"}},
			})
		case r.URL.Path == "/2/domains/domains/test.fr/zones" && r.Method == "GET":
			json.NewEncoder(w).Encode(ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "test.fr"}, {FQDN: "sub.test.fr"}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL
	provider := &Provider{client: client, dryRun: false, domainFilter: nil}

	zone, err := provider.findZoneForEndpoint(context.Background(), endpoint.NewEndpoint("v1.sub.test.fr", "A", "1.2.3.4"))
	require.NoError(t, err)
	assert.Equal(t, "sub.test.fr", zone, "should select the most specific zone")
}

func TestProvider_DeleteRecord_MultiZone(t *testing.T) {
	deletedRecord := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/2/domains/domains" && r.Method == "GET":
			json.NewEncoder(w).Encode(DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "test.fr"}},
			})
		case r.URL.Path == "/2/domains/domains/test.fr/zones" && r.Method == "GET":
			json.NewEncoder(w).Encode(ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "test.fr"}, {FQDN: "sub.test.fr"}},
			})
		case r.URL.Path == "/2/zones/sub.test.fr/records" && r.Method == "GET":
			json.NewEncoder(w).Encode(RecordListResponse{
				Result: "success",
				Data:   []InfomaniakRecord{{ID: 42, Source: "v1", Type: "A", Target: "1.2.3.4", TTL: 60}},
			})
		case r.URL.Path == "/2/zones/sub.test.fr/records/42" && r.Method == "DELETE":
			deletedRecord = true
			json.NewEncoder(w).Encode(APIResponse{Result: "success"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL
	provider := &Provider{client: client, dryRun: false, domainFilter: nil}

	changes := &plan.Changes{
		Delete: []*endpoint.Endpoint{endpoint.NewEndpoint("v1.sub.test.fr", "A", "1.2.3.4")},
	}

	err := provider.ApplyChanges(context.Background(), changes)
	require.NoError(t, err)
	assert.True(t, deletedRecord, "expected DELETE request to sub.test.fr zone")
}

func TestHelperFunctions(t *testing.T) {
	t.Run("recordToEndpoint", func(t *testing.T) {
		record := InfomaniakRecord{Source: "www", Type: "A", Target: "192.0.2.1", TTL: 3600}
		ep := recordToEndpoint(record, "example.com")

		assert.NotNil(t, ep)
		assert.Equal(t, "www.example.com", ep.DNSName)
		assert.Equal(t, "A", ep.RecordType)
		assert.Equal(t, "192.0.2.1", ep.Targets[0])
	})

	t.Run("recordToEndpoint with root record", func(t *testing.T) {
		record := InfomaniakRecord{Source: ".", Type: "A", Target: "192.0.2.1", TTL: 3600}
		ep := recordToEndpoint(record, "example.com")

		assert.NotNil(t, ep)
		assert.Equal(t, "example.com", ep.DNSName)
		assert.Equal(t, "A", ep.RecordType)
	})

	t.Run("ensureFQDN", func(t *testing.T) {
		assert.Equal(t, "example.com", ensureFQDN(".", "example.com"))
		assert.Equal(t, "www.example.com", ensureFQDN("www", "example.com"))
	})

	t.Run("extractRecordSource", func(t *testing.T) {
		assert.Equal(t, ".", extractRecordSource("example.com", "example.com"))
		assert.Equal(t, "www", extractRecordSource("www.example.com", "example.com"))
		assert.Equal(t, "sub.domain", extractRecordSource("sub.domain.example.com", "example.com"))
	})
}
