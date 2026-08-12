package order

// ListOrdersRequest represents the query-object shape used by annot8 tests.
type ListOrdersRequest struct {
	StoreID  *int64  `json:"store_id,omitempty"`
	Status   *string `json:"status,omitempty"`
	Limit    *int    `json:"limit,omitempty"`
	AfterID  *int64  `json:"after_id,omitempty"`
	BeforeID *int64  `json:"before_id,omitempty"`
}
