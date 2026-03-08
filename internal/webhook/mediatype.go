package webhook

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

const (
	contentTypeHeader    = "Content-Type"
	contentTypePlaintext = "text/plain"
	acceptHeader         = "Accept"
	varyHeader           = "Vary"

	// mediaTypeBase is the base media type for the ExternalDNS webhook protocol.
	mediaTypeBase = "application/external.dns.webhook+json"
	// supportedVersion is the webhook API version this provider implements.
	supportedVersion = "1"
	// MediaTypeFormatAndVersion is the full media type with version.
	MediaTypeFormatAndVersion = mediaTypeBase + ";version=" + supportedVersion
)

func checkAcceptHeader(w http.ResponseWriter, r *http.Request) error {
	return checkHeader(w, r, r.Header.Get(acceptHeader), "accept")
}

func checkContentTypeHeader(w http.ResponseWriter, r *http.Request) error {
	return checkHeader(w, r, r.Header.Get(contentTypeHeader), "content-type")
}

func checkHeader(w http.ResponseWriter, r *http.Request, value, headerName string) error {
	if value == "" {
		w.Header().Set(contentTypeHeader, contentTypePlaintext)
		w.WriteHeader(http.StatusNotAcceptable)
		msg := fmt.Sprintf("client must provide a %s header", headerName)
		fmt.Fprint(w, msg)
		return fmt.Errorf("%s", msg)
	}

	ok, unsupportedVersion := negotiate(value)
	if ok {
		return nil
	}

	w.Header().Set(contentTypeHeader, contentTypePlaintext)
	w.WriteHeader(http.StatusUnsupportedMediaType)

	var msg string
	if unsupportedVersion != "" {
		msg = fmt.Sprintf(
			"unsupported webhook API version %q in %s header: this provider only supports version %q",
			unsupportedVersion, headerName, supportedVersion,
		)
	} else {
		msg = fmt.Sprintf("unsupported media type in %s header: %q", headerName, value)
	}

	fmt.Fprint(w, msg)
	log.Print(msg)
	return fmt.Errorf("%s", msg)
}

// negotiate checks whether the header value matches the supported media type.
// Returns (true, "") if supported.
// Returns (false, version) if the base type matches but the version is not supported.
// Returns (false, "") if the media type is entirely different.
func negotiate(header string) (ok bool, unsupportedVersion string) {
	if header == MediaTypeFormatAndVersion {
		return true, ""
	}
	if v, found := strings.CutPrefix(header, mediaTypeBase+";version="); found {
		return false, v
	}
	return false, ""
}
