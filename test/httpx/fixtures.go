package httpx

// ProblemDetails is a portable fixture for problem response annotations.
type ProblemDetails struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
}

// Meta is a portable fixture for paginated response annotations.
type Meta struct {
	Limit int `json:"limit"`
}
