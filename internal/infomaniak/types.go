package infomaniak

// InfomaniakDomain represents a domain from the v2 API
type InfomaniakDomain struct {
	Name string `json:"name"`
}

// InfomaniakZone represents a DNS zone from the v2 API
type InfomaniakZone struct {
	FQDN string `json:"fqdn"`
}

// InfomaniakRecord represents a DNS record from the v2 API
type InfomaniakRecord struct {
	ID       int    `json:"id"`
	Source   string `json:"source"`
	Type     string `json:"type"`
	TTL      int    `json:"ttl"`
	Target   string `json:"target"`
	Priority int    `json:"priority,omitempty"`
}

// APIResponse represents a generic v2 API response
type APIResponse struct {
	Result string      `json:"result"`
	Data   interface{} `json:"data"`
	Error  interface{} `json:"error"`
}

// DomainListResponse represents the response for domains list v2
type DomainListResponse struct {
	Result string             `json:"result"`
	Data   []InfomaniakDomain `json:"data"`
	Error  interface{}        `json:"error"`
}

// ZoneListResponse represents the response for zones list v2
type ZoneListResponse struct {
	Result string           `json:"result"`
	Data   []InfomaniakZone `json:"data"`
	Error  interface{}      `json:"error"`
}

// RecordListResponse represents the response for records list v2
type RecordListResponse struct {
	Result string             `json:"result"`
	Data   []InfomaniakRecord `json:"data"`
	Error  interface{}        `json:"error"`
}

// RecordCreateResponse represents the response when creating a record
type RecordCreateResponse struct {
	Result string           `json:"result"`
	Data   InfomaniakRecord `json:"data"`
	Error  interface{}      `json:"error"`
}

// RecordRequest represents the request body for creating/updating a record
type RecordRequest struct {
	Source   string `json:"source"`
	Type     string `json:"type"`
	Target   string `json:"target"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority,omitempty"`
}
