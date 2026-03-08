package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

type mockProvider struct {
	provider.BaseProvider
	records      []*endpoint.Endpoint
	recordsErr   error
	applyErr     error
	adjustedEps  []*endpoint.Endpoint
	adjustErr    error
	domainFilter endpoint.DomainFilterInterface
}

func (m *mockProvider) Records(_ context.Context) ([]*endpoint.Endpoint, error) {
	return m.records, m.recordsErr
}

func (m *mockProvider) ApplyChanges(_ context.Context, _ *plan.Changes) error {
	return m.applyErr
}

func (m *mockProvider) AdjustEndpoints(eps []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	if m.adjustErr != nil {
		return nil, m.adjustErr
	}
	if m.adjustedEps != nil {
		return m.adjustedEps, nil
	}
	return eps, nil
}

func (m *mockProvider) GetDomainFilter() endpoint.DomainFilterInterface {
	if m.domainFilter != nil {
		return m.domainFilter
	}
	return endpoint.NewDomainFilter([]string{})
}

func TestWebhookNegotiateValid(t *testing.T) {
	wh := New(&mockProvider{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(acceptHeader, MediaTypeFormatAndVersion)

	wh.Negotiate(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, MediaTypeFormatAndVersion, w.Header().Get(contentTypeHeader))
	assert.NotEmpty(t, w.Body.String())
}

func TestWebhookNegotiate_MissingAccept(t *testing.T) {
	wh := New(&mockProvider{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	wh.Negotiate(w, r)

	assert.Equal(t, http.StatusNotAcceptable, w.Code)
}

func TestWebhookNegotiateUnsupportedVersion(t *testing.T) {
	wh := New(&mockProvider{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(acceptHeader, mediaTypeBase+";version=2")

	wh.Negotiate(w, r)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
	assert.Contains(t, w.Body.String(), `"2"`)
	assert.Contains(t, w.Body.String(), `"1"`)
}

func TestWebhookNegotiateReturnsDomainFilter(t *testing.T) {
	filter := endpoint.NewDomainFilter([]string{"test.fr", "sub.test.fr"})
	wh := New(&mockProvider{domainFilter: filter})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(acceptHeader, MediaTypeFormatAndVersion)

	wh.Negotiate(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.NotNil(t, result)
}

func TestWebhookRecordsValid(t *testing.T) {
	eps := []*endpoint.Endpoint{
		endpoint.NewEndpoint("abc.test.fr", "A", "1.2.3.4"),
	}
	wh := New(&mockProvider{records: eps})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/records", nil)
	r.Header.Set(acceptHeader, MediaTypeFormatAndVersion)

	wh.Records(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, MediaTypeFormatAndVersion, w.Header().Get(contentTypeHeader))
	assert.Equal(t, contentTypeHeader, w.Header().Get(varyHeader))

	var result []*endpoint.Endpoint
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	require.Len(t, result, 1)
	assert.Equal(t, "abc.test.fr", result[0].DNSName)
}

func TestWebhookRecordsMissingAccept(t *testing.T) {
	wh := New(&mockProvider{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/records", nil)

	wh.Records(w, r)

	assert.Equal(t, http.StatusNotAcceptable, w.Code)
}

func TestWebhookRecordsProviderError(t *testing.T) {
	wh := New(&mockProvider{recordsErr: errors.New("api failure")})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/records", nil)
	r.Header.Set(acceptHeader, MediaTypeFormatAndVersion)

	wh.Records(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWebhookApplyChangesValid(t *testing.T) {
	wh := New(&mockProvider{})

	changes := plan.Changes{
		Create: []*endpoint.Endpoint{endpoint.NewEndpoint("new.test.fr", "A", "1.2.3.4")},
	}
	body, _ := json.Marshal(changes)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/records", bytes.NewReader(body))
	r.Header.Set(contentTypeHeader, MediaTypeFormatAndVersion)

	wh.ApplyChanges(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestWebhookApplyChangesMissingContentType(t *testing.T) {
	wh := New(&mockProvider{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/records", bytes.NewReader([]byte("{}")))

	wh.ApplyChanges(w, r)

	assert.Equal(t, http.StatusNotAcceptable, w.Code)
}

func TestWebhookApplyChangesInvalidBody(t *testing.T) {
	wh := New(&mockProvider{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/records", bytes.NewReader([]byte("not json")))
	r.Header.Set(contentTypeHeader, MediaTypeFormatAndVersion)

	wh.ApplyChanges(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error decoding changes")
}

func TestWebhookApplyChangesProviderError(t *testing.T) {
	wh := New(&mockProvider{applyErr: errors.New("apply failed")})

	changes := plan.Changes{}
	body, _ := json.Marshal(changes)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/records", bytes.NewReader(body))
	r.Header.Set(contentTypeHeader, MediaTypeFormatAndVersion)

	wh.ApplyChanges(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWebhookAdjustEndpointsValid(t *testing.T) {
	adjusted := []*endpoint.Endpoint{
		endpoint.NewEndpointWithTTL("abc.test.fr", "A", 60, "1.2.3.4"),
	}
	wh := New(&mockProvider{adjustedEps: adjusted})

	input := []*endpoint.Endpoint{
		endpoint.NewEndpointWithTTL("abc.test.fr", "A", 30, "1.2.3.4"),
	}
	body, _ := json.Marshal(input)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/adjustendpoints", bytes.NewReader(body))
	r.Header.Set(contentTypeHeader, MediaTypeFormatAndVersion)
	r.Header.Set(acceptHeader, MediaTypeFormatAndVersion)

	wh.AdjustEndpoints(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, MediaTypeFormatAndVersion, w.Header().Get(contentTypeHeader))
	assert.Equal(t, contentTypeHeader, w.Header().Get(varyHeader))

	var result []*endpoint.Endpoint
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	require.Len(t, result, 1)
	assert.Equal(t, endpoint.TTL(60), result[0].RecordTTL)
}

func TestWebhookAdjustEndpointsMissingContentType(t *testing.T) {
	wh := New(&mockProvider{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/adjustendpoints", bytes.NewReader([]byte("[]")))
	r.Header.Set(acceptHeader, MediaTypeFormatAndVersion)

	wh.AdjustEndpoints(w, r)

	assert.Equal(t, http.StatusNotAcceptable, w.Code)
}

func TestWebhookAdjustEndpointsMissingAccept(t *testing.T) {
	wh := New(&mockProvider{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/adjustendpoints", bytes.NewReader([]byte("[]")))
	r.Header.Set(contentTypeHeader, MediaTypeFormatAndVersion)

	wh.AdjustEndpoints(w, r)

	assert.Equal(t, http.StatusNotAcceptable, w.Code)
}

func TestWebhookAdjustEndpointsInvalidBody(t *testing.T) {
	wh := New(&mockProvider{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/adjustendpoints", bytes.NewReader([]byte("not json")))
	r.Header.Set(contentTypeHeader, MediaTypeFormatAndVersion)
	r.Header.Set(acceptHeader, MediaTypeFormatAndVersion)

	wh.AdjustEndpoints(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "failed to decode request body")
}

func TestWebhookAdjustEndpointsProviderError(t *testing.T) {
	wh := New(&mockProvider{adjustErr: errors.New("adjust failed")})

	body, _ := json.Marshal([]*endpoint.Endpoint{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/adjustendpoints", bytes.NewReader(body))
	r.Header.Set(contentTypeHeader, MediaTypeFormatAndVersion)
	r.Header.Set(acceptHeader, MediaTypeFormatAndVersion)

	wh.AdjustEndpoints(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
