package model

import (
	"time"

	"github.com/google/uuid"
)

// Import statuses.
const (
	ImportUploaded  = "uploaded"
	ImportMapped    = "mapped"
	ImportImporting = "importing"
	ImportDone      = "done"
	ImportFailed    = "failed"
)

// What a file column maps to.
const (
	FieldName      = "name"
	FieldPhone     = "phone"
	FieldEmail     = "email"
	FieldTag       = "tag"
	FieldAttribute = "attribute"
	FieldSkip      = "skip"
)

// Fields are the allowed column mappings.
var Fields = []string{FieldName, FieldPhone, FieldEmail, FieldTag, FieldAttribute, FieldSkip}

// How a preview row would be imported.
const (
	RowNew     = "new"
	RowUpdate  = "update"
	RowInvalid = "invalid"
)

// Import limits (AD-5).
const (
	MaxImportBytes    = 10 << 20
	MaxImportRows     = 50_000
	ImportBatchSize   = 500
	PhoneLookupSize   = 1_000
	PreviewSampleSize = 10
	ColumnSamples     = 3
)

// File types an import accepts.
const (
	FileCSV  = "csv"
	FileXLSX = "xlsx"
)

// UploadedColumn is one column of an uploaded file.
type UploadedColumn struct {
	Name    string
	Samples []string
	Guess   string
}

// Upload is what the mapping step needs to know about a new import.
type Upload struct {
	ID       uuid.UUID
	FileName string
	RowCount int
	Columns  []UploadedColumn
}

// ColumnMapping says what one file column becomes. Stored as JSON on the import.
type ColumnMapping struct {
	Name         string `json:"name"`
	Field        string `json:"field"`
	AttributeKey string `json:"attribute_key,omitempty"`
}

// Mapping is the mapping step's answer.
type Mapping struct {
	Columns        []ColumnMapping
	TagName        *string
	OptInConfirmed bool
}

// Import is an import's progress.
type Import struct {
	ID             uuid.UUID
	FileName       string
	RowCount       int
	Status         string
	Created        int
	Updated        int
	Skipped        int
	Error          *string
	HasSkippedFile bool
	CreatedAt      time.Time
}

// Preview is what starting the import would do.
type Preview struct {
	RowsFound int
	Ready     int
	Updates   int
	Invalid   int
	Sample    []PreviewRow
}

// PreviewRow is one row of the preview sample.
type PreviewRow struct {
	Row    int // 1-based data row (the header is not counted)
	Name   string
	Phone  string
	Email  string
	Status string
	Reason string
}
