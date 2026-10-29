package dto

import (
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
)

type UploadForm struct {
	File huma.FormFile `form:"file" required:"true" doc:".csv or .xlsx, at most 10 MB and 50,000 rows"`
}

type UploadImportInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	RawBody     huma.MultipartFormFiles[UploadForm]
}

type ImportColumnDTO struct {
	Name    string   `json:"name"`
	Samples []string `json:"samples" doc:"Up to 3 values from the column"`
	Guess   string   `json:"guess" enum:"name,phone,email,tag,attribute,skip"`
}

type UploadOutput struct {
	Body struct {
		ID       uuid.UUID         `json:"id"`
		FileName string            `json:"file_name"`
		RowCount int               `json:"row_count"`
		Columns  []ImportColumnDTO `json:"columns"`
	}
}

func ToUploadOutput(u model.Upload) *UploadOutput {
	out := &UploadOutput{}
	out.Body.ID, out.Body.FileName, out.Body.RowCount = u.ID, u.FileName, u.RowCount
	out.Body.Columns = make([]ImportColumnDTO, len(u.Columns))
	for i, c := range u.Columns {
		out.Body.Columns[i] = ImportColumnDTO(c)
	}
	return out
}

type ImportPath struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	ImportID    uuid.UUID `path:"importId"`
}

type ColumnMappingDTO struct {
	Name         string `json:"name" doc:"The file's column name"`
	Field        string `json:"field" enum:"name,phone,email,tag,attribute,skip"`
	AttributeKey string `json:"attribute_key,omitempty" maxLength:"64" doc:"Custom field name for attribute; defaults to the column name"`
}

type MappingInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	ImportID    uuid.UUID `path:"importId"`
	Body        struct {
		Columns        []ColumnMappingDTO `json:"columns" minItems:"1"`
		TagName        *string            `json:"tag_name,omitempty" doc:"A tag for every imported contact"`
		OptInConfirmed bool               `json:"opt_in_confirmed,omitempty" doc:"Must be true: these contacts agreed to get messages"`
	}
}

func (in *MappingInput) ToModel() model.Mapping {
	m := model.Mapping{TagName: in.Body.TagName, OptInConfirmed: in.Body.OptInConfirmed}
	m.Columns = make([]model.ColumnMapping, len(in.Body.Columns))
	for i, c := range in.Body.Columns {
		m.Columns[i] = model.ColumnMapping(c)
	}
	return m
}

type ImportDTO struct {
	ID             uuid.UUID `json:"id"`
	FileName       string    `json:"file_name"`
	RowCount       int       `json:"row_count"`
	Status         string    `json:"status" enum:"uploaded,mapped,importing,done,failed"`
	Created        int       `json:"created"`
	Updated        int       `json:"updated"`
	Skipped        int       `json:"skipped"`
	Error          *string   `json:"error,omitempty"`
	HasSkippedFile bool      `json:"has_skipped_file" doc:"The skipped rows can be downloaded"`
	CreatedAt      time.Time `json:"created_at"`
}

func ToImportDTO(i model.Import) ImportDTO { return ImportDTO(i) }

type ImportOutput struct {
	Body ImportDTO
}

type PreviewRowDTO struct {
	Row    int    `json:"row" doc:"1-based data row; the header isn't counted"`
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Email  string `json:"email"`
	Status string `json:"status" enum:"new,update,invalid"`
	Reason string `json:"reason,omitempty" doc:"Why the row is invalid"`
}

type PreviewOutput struct {
	Body struct {
		RowsFound int             `json:"rows_found"`
		Ready     int             `json:"ready" doc:"New contacts"`
		Updates   int             `json:"updates" doc:"Rows whose phone is already a contact"`
		Invalid   int             `json:"invalid"`
		Sample    []PreviewRowDTO `json:"sample" doc:"The first 10 rows"`
	}
}

func ToPreviewOutput(p model.Preview) *PreviewOutput {
	out := &PreviewOutput{}
	out.Body.RowsFound, out.Body.Ready, out.Body.Updates, out.Body.Invalid = p.RowsFound, p.Ready, p.Updates, p.Invalid
	out.Body.Sample = make([]PreviewRowDTO, len(p.Sample))
	for i, r := range p.Sample {
		out.Body.Sample[i] = PreviewRowDTO(r)
	}
	return out
}
