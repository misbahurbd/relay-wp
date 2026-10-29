package model

// Message template statuses, as other features see them (Meta's states, lowercased).
const (
	TemplatePending  = "pending"
	TemplateApproved = "approved"
	TemplateRejected = "rejected"
	TemplatePaused   = "paused"
	TemplateDisabled = "disabled"
)

// TemplateSpec is a message template's content in plain terms. The Graph client
// converts it to and from Meta's components format.
type TemplateSpec struct {
	Name     string
	Language string
	// Category is marketing, utility or authentication.
	Category string
	Header   TemplateHeader
	Body     TemplateBody
	Footer   string
	Buttons  []TemplateButton
}

// TemplateHeader is the optional header.
type TemplateHeader struct {
	// Format is none, text, image, video or document.
	Format string
	// Text and Example are for a text header (Example fills its {{1}}).
	Text    string
	Example string
	// Handle is the Resumable Upload handle of a media header's sample.
	Handle string
}

// TemplateBody is the body text and one example per {{n}} variable.
type TemplateBody struct {
	Text     string
	Examples []string
}

// TemplateButton is a quick_reply, url or phone button. Value is the link or number.
type TemplateButton struct {
	Type  string
	Text  string
	Value string
}

// MessageTemplate is a template on the WhatsApp Business account.
type MessageTemplate struct {
	ID             string
	Status         string
	RejectedReason string
	Spec           TemplateSpec
}
