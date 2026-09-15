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

func TestProviderRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/2/domains/domains":
			response := DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "example.com"}},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case "/2/domains/domains/example.com/zones":
			response := ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "example.com"}},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case "/2/zones/example.com/records":
			response := RecordListResponse{
				Result: "success",
				Data: []InfomaniakRecord{
					{ID: 1, Source: ".", Type: "A", Target: "192.0.2.1", TTL: 3600},
					{ID: 2, Source: "www", Type: "CNAME", Target: "example.com", TTL: 1800},
				},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
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

func TestProviderApplyChangesDryRun(t *testing.T) {
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

func TestProviderCreateRecord(t *testing.T) {
	createdRecord := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/2/domains/domains" && r.Method == "GET":
			response := DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "example.com"}},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case r.URL.Path == "/2/domains/domains/example.com/zones" && r.Method == "GET":
			response := ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "example.com"}},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case r.URL.Path == "/2/zones/example.com/records" && r.Method == "POST":
			createdRecord = true
			var req RecordRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
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
			require.NoError(t, json.NewEncoder(w).Encode(response))
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

func TestProviderDeleteRecord(t *testing.T) {
	deletedRecord := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/2/domains/domains" && r.Method == "GET":
			response := DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "example.com"}},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case r.URL.Path == "/2/domains/domains/example.com/zones" && r.Method == "GET":
			response := ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "example.com"}},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case r.URL.Path == "/2/zones/example.com/records" && r.Method == "GET":
			response := RecordListResponse{
				Result: "success",
				Data:   []InfomaniakRecord{{ID: 1, Source: "test", Type: "A", Target: "192.0.2.1", TTL: 3600}},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case r.URL.Path == "/2/zones/example.com/records/1" && r.Method == "DELETE":
			deletedRecord = true
			response := APIResponse{Result: "success"}
			require.NoError(t, json.NewEncoder(w).Encode(response))
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

func TestProviderAdjustEndpoints(t *testing.T) {
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

func TestProviderMostSpecificZone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/2/domains/domains" && r.Method == "GET":
			response := DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "test.fr"}},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case r.URL.Path == "/2/domains/domains/test.fr/zones" && r.Method == "GET":
			response := ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "test.fr"}, {FQDN: "sub.test.fr"}},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
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

func TestProviderDeleteRecordMultiZone(t *testing.T) {
	deletedRecord := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/2/domains/domains" && r.Method == "GET":
			response := DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "test.fr"}},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case r.URL.Path == "/2/domains/domains/test.fr/zones" && r.Method == "GET":
			response := ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "test.fr"}, {FQDN: "sub.test.fr"}},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case r.URL.Path == "/2/zones/sub.test.fr/records" && r.Method == "GET":
			response := RecordListResponse{
				Result: "success",
				Data:   []InfomaniakRecord{{ID: 42, Source: "v1", Type: "A", Target: "1.2.3.4", TTL: 60}},
			}
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case r.URL.Path == "/2/zones/sub.test.fr/records/42" && r.Method == "DELETE":
			deletedRecord = true
			response := APIResponse{Result: "success"}
			require.NoError(t, json.NewEncoder(w).Encode(response))
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

func TestNormalizeReadTarget(t *testing.T) {
	tests := []struct {
		name       string
		recordType string
		target     string
		want       string
	}{
		{"SRV without trailing dot gets one", "SRV", "10 50 3478 turn.example.com", "10 50 3478 turn.example.com."},
		{"SRV already dotted is unchanged", "SRV", "10 50 3478 turn.example.com.", "10 50 3478 turn.example.com."},
		{"TXT surrounding quotes stripped", "TXT", "\"v=DMARC1; p=quarantine\"", "v=DMARC1; p=quarantine"},
		{"TXT chunked value is concatenated", "TXT", "\"chunk-one-\" \"chunk-two\"", "chunk-one-chunk-two"},
		{"TXT escaped quote is preserved", "TXT", "\"a\\\"b\"", "a\"b"},
		{"TXT without quotes is unchanged", "TXT", "v=spf1 -all", "v=spf1 -all"},
		{"A record is untouched", "A", "192.0.2.1", "192.0.2.1"},
		{"CNAME record is untouched", "CNAME", "target.example.com.", "target.example.com."},
		{"MX record is untouched", "MX", "10 mail.example.com.", "10 mail.example.com."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeReadTarget(tt.recordType, tt.target))
		})
	}
}

func TestRecordToEndpointNormalizesTargets(t *testing.T) {
	srv := recordToEndpoint(InfomaniakRecord{Source: "_sip._tcp", Type: "SRV", Target: "10 50 3478 turn.example.com", TTL: 3600}, "example.com")
	require.NotNil(t, srv)
	assert.Equal(t, "10 50 3478 turn.example.com.", srv.Targets[0])

	txt := recordToEndpoint(InfomaniakRecord{Source: "_dmarc", Type: "TXT", Target: "\"v=DMARC1; p=quarantine\"", TTL: 3600}, "example.com")
	require.NotNil(t, txt)
	assert.Equal(t, "v=DMARC1; p=quarantine", txt.Targets[0])
}

func TestProviderRecordsNormalizesSRVAndTXT(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/2/domains/domains":
			require.NoError(t, json.NewEncoder(w).Encode(DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "example.com"}},
			}))
		case "/2/domains/domains/example.com/zones":
			require.NoError(t, json.NewEncoder(w).Encode(ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "example.com"}},
			}))
		case "/2/zones/example.com/records":
			// The Infomaniak API returns SRV targets without a trailing dot and TXT
			// values wrapped in literal quotes; Records() must normalize both so the
			// endpoints match what ExternalDNS holds (otherwise they churn forever).
			require.NoError(t, json.NewEncoder(w).Encode(RecordListResponse{
				Result: "success",
				Data: []InfomaniakRecord{
					{ID: 1, Source: "_sip._tcp", Type: "SRV", Target: "10 50 3478 turn.example.com", TTL: 3600},
					{ID: 2, Source: "_dmarc", Type: "TXT", Target: "\"v=DMARC1; p=quarantine\"", TTL: 3600},
				},
			}))
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
	require.Len(t, endpoints, 2)

	byType := map[string]*endpoint.Endpoint{}
	for _, ep := range endpoints {
		byType[ep.RecordType] = ep
	}

	require.Contains(t, byType, "SRV")
	assert.Equal(t, "10 50 3478 turn.example.com.", byType["SRV"].Targets[0])

	require.Contains(t, byType, "TXT")
	assert.Equal(t, "v=DMARC1; p=quarantine", byType["TXT"].Targets[0])
}

func TestProviderRecordsMergesMultiTargetRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/2/domains/domains":
			require.NoError(t, json.NewEncoder(w).Encode(DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "example.com"}},
			}))
		case "/2/domains/domains/example.com/zones":
			require.NoError(t, json.NewEncoder(w).Encode(ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "example.com"}},
			}))
		case "/2/zones/example.com/records":
			// Infomaniak stores one row per target; a name with several targets
			// (round-robin A, multiple MX) and a long TXT split into 255-byte chunks
			// must collapse into one endpoint each, or they churn every reconcile.
			require.NoError(t, json.NewEncoder(w).Encode(RecordListResponse{
				Result: "success",
				Data: []InfomaniakRecord{
					{ID: 1, Source: ".", Type: "A", Target: "192.0.2.1", TTL: 300},
					{ID: 2, Source: ".", Type: "A", Target: "192.0.2.2", TTL: 300},
					{ID: 3, Source: ".", Type: "MX", Target: "10 mx1.example.com", TTL: 300},
					{ID: 4, Source: ".", Type: "MX", Target: "10 mx2.example.com", TTL: 300},
					{ID: 5, Source: "sel._domainkey", Type: "TXT", Target: "\"part-one-\" \"part-two\"", TTL: 300},
				},
			}))
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

	byType := map[string]*endpoint.Endpoint{}
	for _, ep := range endpoints {
		byType[ep.RecordType] = ep
	}

	// One endpoint per (name, type), not one per row.
	require.Len(t, endpoints, 3)

	require.Contains(t, byType, "A")
	assert.ElementsMatch(t, []string{"192.0.2.1", "192.0.2.2"}, byType["A"].Targets)

	require.Contains(t, byType, "MX")
	assert.ElementsMatch(t, []string{"10 mx1.example.com", "10 mx2.example.com"}, byType["MX"].Targets)

	require.Contains(t, byType, "TXT")
	require.Len(t, byType["TXT"].Targets, 1)
	assert.Equal(t, "part-one-part-two", byType["TXT"].Targets[0])
}

func TestProviderUpdateRecordReconcilesTargets(t *testing.T) {
	var created []RecordRequest
	var deleted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/2/domains/domains" && r.Method == "GET":
			require.NoError(t, json.NewEncoder(w).Encode(DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "example.com"}},
			}))
		case r.URL.Path == "/2/domains/domains/example.com/zones" && r.Method == "GET":
			require.NoError(t, json.NewEncoder(w).Encode(ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "example.com"}},
			}))
		case r.URL.Path == "/2/zones/example.com/records" && r.Method == "GET":
			require.NoError(t, json.NewEncoder(w).Encode(RecordListResponse{
				Result: "success",
				Data: []InfomaniakRecord{
					{ID: 10, Source: "www", Type: "A", Target: "192.0.2.1", TTL: 300},
					{ID: 11, Source: "www", Type: "A", Target: "192.0.2.2", TTL: 300},
				},
			}))
		case r.URL.Path == "/2/zones/example.com/records" && r.Method == "POST":
			var req RecordRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			created = append(created, req)
			require.NoError(t, json.NewEncoder(w).Encode(RecordCreateResponse{Result: "success", Data: InfomaniakRecord{ID: 12}}))
		case r.Method == "DELETE":
			deleted = append(deleted, r.URL.Path)
			require.NoError(t, json.NewEncoder(w).Encode(APIResponse{Result: "success"}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL
	provider := &Provider{client: client, dryRun: false, domainFilter: nil}

	// Current targets {192.0.2.1, 192.0.2.2}; desired {192.0.2.2, 192.0.2.3}.
	changes := &plan.Changes{
		UpdateOld: []*endpoint.Endpoint{endpoint.NewEndpoint("www.example.com", "A", "192.0.2.1", "192.0.2.2")},
		UpdateNew: []*endpoint.Endpoint{endpoint.NewEndpoint("www.example.com", "A", "192.0.2.2", "192.0.2.3")},
	}
	require.NoError(t, provider.ApplyChanges(context.Background(), changes))

	// Only the new target is created and only the removed target's row deleted;
	// the shared target (192.0.2.2, id 11) is left untouched.
	require.Len(t, created, 1)
	assert.Equal(t, "192.0.2.3", created[0].Target)
	assert.Equal(t, "www", created[0].Source)
	require.Len(t, deleted, 1)
	assert.Equal(t, "/2/zones/example.com/records/10", deleted[0])
}

func TestProviderUpdateRecordRespectsOwnership(t *testing.T) {
	var deleted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/2/domains/domains" && r.Method == "GET":
			require.NoError(t, json.NewEncoder(w).Encode(DomainListResponse{
				Result: "success",
				Data:   []InfomaniakDomain{{Name: "example.com"}},
			}))
		case r.URL.Path == "/2/domains/domains/example.com/zones" && r.Method == "GET":
			require.NoError(t, json.NewEncoder(w).Encode(ZoneListResponse{
				Result: "success",
				Data:   []InfomaniakZone{{FQDN: "example.com"}},
			}))
		case r.URL.Path == "/2/zones/example.com/records" && r.Method == "GET":
			require.NoError(t, json.NewEncoder(w).Encode(RecordListResponse{
				Result: "success",
				Data: []InfomaniakRecord{
					{ID: 10, Source: "www", Type: "A", Target: "192.0.2.1", TTL: 300},
					{ID: 11, Source: "www", Type: "A", Target: "192.0.2.2", TTL: 300},
					{ID: 99, Source: "www", Type: "A", Target: "203.0.113.9", TTL: 300},
				},
			}))
		case r.URL.Path == "/2/zones/example.com/records" && r.Method == "POST":
			require.NoError(t, json.NewEncoder(w).Encode(RecordCreateResponse{Result: "success", Data: InfomaniakRecord{ID: 12}}))
		case r.Method == "DELETE":
			deleted = append(deleted, r.URL.Path)
			require.NoError(t, json.NewEncoder(w).Encode(APIResponse{Result: "success"}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &Config{APIToken: "test-token", DryRun: false}
	client := NewInfomaniakClient(config)
	client.baseURL = server.URL
	provider := &Provider{client: client, dryRun: false, domainFilter: nil}

	// A third row (id 99) exists live but was never owned by ExternalDNS (it is not
	// in UpdateOld) — e.g. added by another tool managing the same zone. It must
	// survive the reconcile: deletions come from oldEp, not the live records.
	changes := &plan.Changes{
		UpdateOld: []*endpoint.Endpoint{endpoint.NewEndpoint("www.example.com", "A", "192.0.2.1", "192.0.2.2")},
		UpdateNew: []*endpoint.Endpoint{endpoint.NewEndpoint("www.example.com", "A", "192.0.2.2", "192.0.2.3")},
	}
	require.NoError(t, provider.ApplyChanges(context.Background(), changes))

	// Only ExternalDNS's own removed target (192.0.2.1, id 10) is deleted; the
	// unowned row (id 99) is left untouched.
	require.Len(t, deleted, 1)
	assert.Equal(t, "/2/zones/example.com/records/10", deleted[0])
}
