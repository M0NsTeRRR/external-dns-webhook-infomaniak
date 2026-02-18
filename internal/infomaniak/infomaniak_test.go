package infomaniak

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

// TestNewInfomaniakProvider tests the provider initialization
func TestNewInfomaniakProvider(t *testing.T) {
	config := &Config{
		APIToken: "test-token",
		Debug:    true,
		DryRun:   false,
	}

	provider := NewInfomaniakProvider(nil, config)

	assert.NotNil(t, provider)
	assert.NotNil(t, provider.client)
	assert.Equal(t, config.DryRun, provider.dryRun)
	assert.Equal(t, config, provider.client.config)
}

// TestProvider_ApplyChanges_DryRun tests ApplyChanges in dry-run mode
func TestProvider_ApplyChanges_DryRun(t *testing.T) {
	config := &Config{
		APIToken: "test-token",
		Debug:    false,
		DryRun:   true,
	}
	client := NewInfomaniakClient(config)

	provider := &Provider{
		client:       client,
		dryRun:       true,
		domainFilter: nil,
	}

	changes := &plan.Changes{
		Delete: []*endpoint.Endpoint{
			endpoint.NewEndpoint("test.example.com", "A", "192.0.2.1"),
		},
		Create: []*endpoint.Endpoint{
			endpoint.NewEndpoint("new.example.com", "A", "192.0.2.2"),
		},
	}

	// No API calls should be made in dry-run mode
	err := provider.ApplyChanges(context.Background(), changes)

	require.NoError(t, err)
}

// TestHelperFunctions tests the helper functions
func TestHelperFunctions(t *testing.T) {
	t.Run("recordToEndpoint", func(t *testing.T) {
		record := InfomaniakRecord{
			Name:    "www",
			Type:    "A",
			Content: "192.0.2.1",
			TTL:     3600,
		}

		endpoint := recordToEndpoint(record, "example.com")

		assert.NotNil(t, endpoint)
		assert.Equal(t, "www.example.com", endpoint.DNSName)
		assert.Equal(t, "A", endpoint.RecordType)
		assert.Equal(t, "192.0.2.1", endpoint.Targets[0])
	})

	t.Run("recordToEndpoint with root record", func(t *testing.T) {
		record := InfomaniakRecord{
			Name:    "@",
			Type:    "A",
			Content: "192.0.2.1",
			TTL:     3600,
		}

		endpoint := recordToEndpoint(record, "example.com")

		assert.NotNil(t, endpoint)
		assert.Equal(t, "example.com", endpoint.DNSName)
		assert.Equal(t, "A", endpoint.RecordType)
	})

	t.Run("ensureFQDN", func(t *testing.T) {
		assert.Equal(t, "example.com", ensureFQDN("@", "example.com"))
		assert.Equal(t, "example.com", ensureFQDN("example.com", "example.com"))
		assert.Equal(t, "www.example.com", ensureFQDN("www", "example.com"))
		assert.Equal(t, "sub.example.com", ensureFQDN("sub.example.com", "example.com"))
	})
}
