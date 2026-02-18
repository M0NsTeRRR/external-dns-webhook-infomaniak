package infomaniak

// InfomaniakAccount represents an Infomaniak account
type InfomaniakAccount struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// InfomaniakZone represents a DNS zone
type InfomaniakZone struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// InfomaniakRecord represents a DNS record
type InfomaniakRecord struct {
	ID      int    `json:"id"`
	ZoneID  int    `json:"zone_id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Pri     int    `json:"pri"`
}

// APIResponse represents a generic API response
type APIResponse struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

// AccountListResponse represents the response for the accounts list
type AccountListResponse struct {
	Data  []InfomaniakAccount `json:"data"`
	Error string              `json:"error"`
}

// ZoneListResponse represents the response for the zones list
type ZoneListResponse struct {
	Data  []InfomaniakZone `json:"data"`
	Error string           `json:"error"`
}

// RecordListResponse represents the response for the records list
type RecordListResponse struct {
	Data  []InfomaniakRecord `json:"data"`
	Error string             `json:"error"`
}
