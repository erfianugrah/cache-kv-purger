package api

// Error represents an error returned by the Cloudflare API
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// APIResponse is a generic response from the Cloudflare API
type APIResponse struct {
	Success  bool     `json:"success"`
	Errors   []Error  `json:"errors"`
	Messages []string `json:"messages"`
}

// PaginationInfo contains information about pagination
type PaginationInfo struct {
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
	TotalPages int    `json:"total_pages"`
	Count      int    `json:"count"`
	Total      int    `json:"total_count"`
	Cursor     string `json:"cursor"`
}

// PaginatedResponse is a response that includes pagination information
type PaginatedResponse struct {
	APIResponse
	ResultInfo PaginationInfo `json:"result_info"`
}

// Zone represents a Cloudflare zone
type Zone struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	NameServers []string `json:"name_servers,omitempty"`
	Type        string   `json:"type,omitempty"`
}

// ZonesResponse represents the response from a zones list request
type ZonesResponse struct {
	PaginatedResponse
	Result []Zone `json:"result"`
}
