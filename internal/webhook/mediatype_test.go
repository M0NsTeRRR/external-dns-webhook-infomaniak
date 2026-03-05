package webhook

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNegotiate(t *testing.T) {
	tests := []struct {
		name                string
		header              string
		wantOK              bool
		wantUnsupportedVer  string
	}{
		{
			name:   "exact match",
			header: MediaTypeFormatAndVersion,
			wantOK: true,
		},
		{
			name:   "wildcard",
			header: "*/*",
			wantOK: true,
		},
		{
			name:   "multiple values with exact match",
			header: "application/json, " + MediaTypeFormatAndVersion,
			wantOK: true,
		},
		{
			name:   "with quality value on supported type",
			header: MediaTypeFormatAndVersion + ";q=0.9",
			wantOK: true,
		},
		{
			name:   "multiple values with wildcard fallback",
			header: "application/json;q=0.8, */*;q=0.1",
			wantOK: true,
		},
		{
			name:               "known base type but unsupported version",
			header:             mediaTypeBase + ";version=2",
			wantOK:             false,
			wantUnsupportedVer: "2",
		},
		{
			name:               "multiple values, unsupported version only",
			header:             mediaTypeBase + ";version=2, " + mediaTypeBase + ";version=3",
			wantOK:             false,
			wantUnsupportedVer: "3",
		},
		{
			name:   "completely different media type",
			header: "application/json",
			wantOK: false,
		},
		{
			name:   "empty header",
			header: "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, unsupportedVer := negotiate(tt.header)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantUnsupportedVer, unsupportedVer)
		})
	}
}

func TestCheckAcceptHeader(t *testing.T) {
	tests := []struct {
		name           string
		accept         string
		wantStatus     int
		wantErrContain string
	}{
		{
			name:       "valid accept header",
			accept:     MediaTypeFormatAndVersion,
			wantStatus: http.StatusOK,
		},
		{
			name:           "missing accept header",
			accept:         "",
			wantStatus:     http.StatusNotAcceptable,
			wantErrContain: "client must provide a accept header",
		},
		{
			name:           "unsupported version",
			accept:         mediaTypeBase + ";version=2",
			wantStatus:     http.StatusUnsupportedMediaType,
			wantErrContain: `unsupported webhook API version "2"`,
		},
		{
			name:           "completely different media type",
			accept:         "application/json",
			wantStatus:     http.StatusUnsupportedMediaType,
			wantErrContain: "unsupported media type in accept header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.accept != "" {
				r.Header.Set(acceptHeader, tt.accept)
			}

			err := checkAcceptHeader(w, r)

			if tt.wantStatus == http.StatusOK {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Equal(t, tt.wantStatus, w.Code)
				assert.Contains(t, w.Body.String(), tt.wantErrContain)
				assert.Equal(t, contentTypePlaintext, w.Header().Get(contentTypeHeader))
			}
		})
	}
}

func TestCheckContentTypeHeader(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		wantStatus     int
		wantErrContain string
	}{
		{
			name:        "valid content-type",
			contentType: MediaTypeFormatAndVersion,
			wantStatus:  http.StatusOK,
		},
		{
			name:           "missing content-type",
			contentType:    "",
			wantStatus:     http.StatusNotAcceptable,
			wantErrContain: "client must provide a content-type header",
		},
		{
			name:           "unsupported version",
			contentType:    mediaTypeBase + ";version=2",
			wantStatus:     http.StatusUnsupportedMediaType,
			wantErrContain: `unsupported webhook API version "2"`,
		},
		{
			name:           "completely different media type",
			contentType:    "application/json",
			wantStatus:     http.StatusUnsupportedMediaType,
			wantErrContain: "unsupported media type in content-type header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/records", nil)
			if tt.contentType != "" {
				r.Header.Set(contentTypeHeader, tt.contentType)
			}

			err := checkContentTypeHeader(w, r)

			if tt.wantStatus == http.StatusOK {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Equal(t, tt.wantStatus, w.Code)
				assert.Contains(t, w.Body.String(), tt.wantErrContain)
			}
		})
	}
}
