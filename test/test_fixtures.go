package annot8fixtures

// TestSimple is a helper struct used by openapi tests to verify schema generation.
type TestSimple struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TestWithPointer exercises pointer field handling in schema generation.
type TestWithPointer struct {
	Name *string `json:"name,omitempty"`
}

// TestWithArray exercises array field handling in schema generation.
type TestWithArray struct {
	Tags []string `json:"tags"`
}

// TestJSONIgnoredField verifies json:"-" fields are omitted from generated schemas.
type TestJSONIgnoredField struct {
	Visible string `json:"visible"`
	Err     error  `json:"-"`
}

// TestNested verifies nested struct references across generated schemas.
type TestNested struct {
	Simple TestSimple `json:"simple"`
}

// testHelperOther is an auxiliary struct referenced by TestWithQualified.
type testHelperOther struct {
	Foo int `json:"foo"`
}

// TestWithQualified ensures qualified type names are generated for nested structs.
type TestWithQualified struct {
	Other testHelperOther `json:"other"`
}

// TagExample exercises enhanced tag handling when generating schemas.
type TagExample struct {
	ID    string `json:"id"              openapi:"format=uuid,deprecated=true,default=00000000-0000-0000-0000-000000000000"`
	Alias string `json:"alias,omitempty" openapi:"pattern=^a.*$,minLength=2,maxLength=5"`
	Email string `json:"email"                                                                                              validate:"email"`
	Owner string `json:"owner,omitempty"                                                                                                     binding:"uuid"`
}

// TagOneOfExample verifies validate tags containing spaces are parsed correctly.
type TagOneOfExample struct {
	Sort string `json:"sort" validate:"oneof=asc desc"`
}

// MyEnum is a test enum representing string-based constants.
type MyEnum string

const (
	MyEnumA MyEnum = "A"
	MyEnumB MyEnum = "B"
)

// TestWithEnumField verifies that enum fields in structs are properly referenced
type TestWithEnumField struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status MyEnum `json:"status"`
}

// TestCompliance31 verifies OpenAPI 3.1 specific schema features.
type TestCompliance31 struct {
	Count int     `json:"count" openapi:"exclusiveMin=0,exclusiveMax=100"`
	Rate  float64 `json:"rate"  openapi:"exclusiveMin=0.5"`
}

// TestAliasMap is a type alias for a map.
type TestAliasMap map[string]int

// TestAliasSlice is a type alias for a slice.
type TestAliasSlice []TestSimple

// TestUUIDTextMarshaler exercises TextMarshaler-based schema inference.
type TestUUIDTextMarshaler struct{}

func (TestUUIDTextMarshaler) MarshalText() ([]byte, error) {
	return []byte{}, nil
}

func (*TestUUIDTextMarshaler) UnmarshalText([]byte) error {
	return nil
}

// TestEmailJSONMarshaler exercises JSON marshaler string+format inference from type name.
type TestEmailJSONMarshaler struct {
	Value string `json:"value"`
}

func (TestEmailJSONMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(`"test@example.com"`), nil
}

func (*TestEmailJSONMarshaler) UnmarshalJSON([]byte) error {
	return nil
}

// TestTimestampJSONMarshaler exercises date-time format inference.
type TestTimestampJSONMarshaler struct {
	Seconds int64 `json:"seconds"`
}

func (TestTimestampJSONMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(`"2026-01-01T00:00:00Z"`), nil
}

func (*TestTimestampJSONMarshaler) UnmarshalJSON([]byte) error {
	return nil
}
