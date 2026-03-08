package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	extdnsprovider "sigs.k8s.io/external-dns/provider"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

// Webhook holds the DNS provider and exposes HTTP handlers for ExternalDNS.
type Webhook struct {
	provider extdnsprovider.Provider
}

func New(p extdnsprovider.Provider) *Webhook {
	return &Webhook{provider: p}
}

// Negotiate handles GET / — reads the Accept header, validates it,
// sets Content-Type, and returns the provider's DomainFilter.
func (wh *Webhook) Negotiate(w http.ResponseWriter, r *http.Request) {
	if err := checkAcceptHeader(w, r); err != nil {
		return
	}
	b, err := json.Marshal(wh.provider.GetDomainFilter())
	if err != nil {
		log.Printf("failed to marshal domain filter: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentTypeHeader, MediaTypeFormatAndVersion)
	if _, err := w.Write(b); err != nil {
		log.Printf("error writing negotiate response: %v", err)
	}
}

// Records handles GET /records — validates Accept and returns current DNS records.
func (wh *Webhook) Records(w http.ResponseWriter, r *http.Request) {
	if err := checkAcceptHeader(w, r); err != nil {
		return
	}
	records, err := wh.provider.Records(r.Context())
	if err != nil {
		log.Printf("error getting records: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentTypeHeader, MediaTypeFormatAndVersion)
	w.Header().Set(varyHeader, contentTypeHeader)
	if err := json.NewEncoder(w).Encode(records); err != nil {
		log.Printf("error encoding records: %v", err)
	}
}

// ApplyChanges handles POST /records — validates Content-Type and applies DNS changes.
func (wh *Webhook) ApplyChanges(w http.ResponseWriter, r *http.Request) {
	if err := checkContentTypeHeader(w, r); err != nil {
		return
	}
	var changes plan.Changes
	if err := json.NewDecoder(r.Body).Decode(&changes); err != nil {
		w.Header().Set(contentTypeHeader, contentTypePlaintext)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "error decoding changes: %v", err)
		return
	}
	if err := wh.provider.ApplyChanges(context.Background(), &changes); err != nil {
		log.Printf("error applying changes: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AdjustEndpoints handles POST /adjustendpoints — validates both Content-Type and Accept.
func (wh *Webhook) AdjustEndpoints(w http.ResponseWriter, r *http.Request) {
	if err := checkContentTypeHeader(w, r); err != nil {
		return
	}
	if err := checkAcceptHeader(w, r); err != nil {
		return
	}
	var pve []*endpoint.Endpoint
	if err := json.NewDecoder(r.Body).Decode(&pve); err != nil {
		w.Header().Set(contentTypeHeader, contentTypePlaintext)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "failed to decode request body: %v", err)
		return
	}
	pve, err := wh.provider.AdjustEndpoints(pve)
	if err != nil {
		log.Printf("error adjusting endpoints: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentTypeHeader, MediaTypeFormatAndVersion)
	w.Header().Set(varyHeader, contentTypeHeader)
	if err := json.NewEncoder(w).Encode(pve); err != nil {
		log.Printf("error encoding adjusted endpoints: %v", err)
	}
}
