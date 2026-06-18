package api

// Stage is the lifecycle stage of a clip project, reported by the API on every
// GetProject. There is no numeric percentage, so progress is presented as a
// stepper over StageOrder.
type Stage string

// Project stages (OpenAPI ClipProjectRepresentation.stage enum). PENDING…COMPLETE
// is the happy path; STALLED is the terminal failure state.
const (
	StagePending  Stage = "PENDING"
	StageQueued   Stage = "QUEUED"
	StageImport   Stage = "IMPORT"
	StageCurate   Stage = "CURATE"
	StageRefine   Stage = "REFINE"
	StageRender   Stage = "RENDER"
	StageUpload   Stage = "UPLOAD"
	StageComplete Stage = "COMPLETE"
	StageStalled  Stage = "STALLED"
)

// StageOrder is the ordered happy-path sequence used to render the progress
// stepper. STALLED is deliberately excluded — it is a failure, not a step.
var StageOrder = []Stage{
	StagePending, StageQueued, StageImport, StageCurate,
	StageRefine, StageRender, StageUpload, StageComplete,
}

// IsTerminal reports whether the stage is an end state (success or failure).
func (s Stage) IsTerminal() bool { return s == StageComplete || s == StageStalled }

// Index returns the position of s within StageOrder, or -1 if s is not a
// happy-path stage (e.g. STALLED or an unknown future value).
func (s Stage) Index() int {
	for i, st := range StageOrder {
		if st == s {
			return i
		}
	}
	return -1
}

// Project is a clip project (OpenAPI ClipProjectRepresentation). Only the fields
// the CLI surfaces are modeled; unknown fields decode away harmlessly.
type Project struct {
	ID             string `json:"id"`
	ProjectID      string `json:"projectId"`
	Stage          Stage  `json:"stage"`
	Error          string `json:"error,omitempty"`
	SourcePlatform string `json:"sourcePlatform,omitempty"`
	SourceID       string `json:"sourceId,omitempty"`
	SourceURI      string `json:"sourceUri,omitempty"`
	Model          string `json:"model,omitempty"`
	Visibility     string `json:"visibility,omitempty"`
	CreatedAt      string `json:"createdAt,omitempty"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
}

// CreateProjectInput is the body for POST /api/clip-projects (OpenAPI
// CreateClipProjectCommand). Only VideoURL is required.
type CreateProjectInput struct {
	VideoURL          string              `json:"videoUrl"`
	ConclusionActions []ConclusionAction  `json:"conclusionActions,omitempty"`
	CurationPref      *CurationPreference `json:"curationPref,omitempty"`
	ImportPref        *ImportPreference   `json:"importPref,omitempty"`
	BrandTemplateID   string              `json:"brandTemplateId,omitempty"`
}

// ConclusionAction fires on project conclusion: a webhook POST or an email.
type ConclusionAction struct {
	Type          string `json:"type"` // EMAIL | WEBHOOK
	URL           string `json:"url,omitempty"`
	Email         string `json:"email,omitempty"`
	NotifyFailure bool   `json:"notifyFailure,omitempty"`
}

// CurationPreference controls how the source video is curated into clips.
type CurationPreference struct {
	ClipDurations [][]int        `json:"clipDurations,omitempty"` // [[minSec,maxSec],…]
	Genre         string         `json:"genre,omitempty"`
	Range         *CurationRange `json:"range,omitempty"`
	TopicKeywords []string       `json:"topicKeywords,omitempty"`
}

// CurationRange restricts curation to a sub-range of the source video.
type CurationRange struct {
	StartSec float64 `json:"startSec,omitempty"`
	EndSec   float64 `json:"endSec,omitempty"`
}

// ImportPreference controls source import (e.g. spoken-language detection).
type ImportPreference struct {
	SourceLang string `json:"sourceLang,omitempty"`
}

// ExportableClip is a generated clip (OpenAPI ExportableClipRepresentation).
//
// The Score* fields are NOT part of the public OpenAPI spec; the live API and
// the first-party CLI surface virality/judge scores, so they are modeled as
// optional best-effort pointers — absent in the spec is fine, present is shown.
type ExportableClip struct {
	ID            string      `json:"id"` // "{projectId}.{curationId}"
	ProjectID     string      `json:"projectId"`
	CurationID    string      `json:"curationId,omitempty"`
	Title         string      `json:"title"`
	Description   string      `json:"description,omitempty"`
	Hashtags      string      `json:"hashtags,omitempty"`
	DurationMs    float64     `json:"durationMs,omitempty"`
	TimeRanges    [][]float64 `json:"timeRanges,omitempty"`
	Keywords      []string    `json:"keywords,omitempty"`
	Genre         string      `json:"genre,omitempty"`
	Subgenre      string      `json:"subgenre,omitempty"`
	URIForPreview string      `json:"uriForPreview,omitempty"`
	URIForExport  string      `json:"uriForExport,omitempty"` // GCS signed URL (no auth) for download
	CreatedAt     string      `json:"createdAt,omitempty"`

	// Unofficial, best-effort virality/judge scores (not in the OpenAPI spec).
	Score         *float64 `json:"score,omitempty"`
	HookScore     *float64 `json:"hookScore,omitempty"`
	ViralityScore *float64 `json:"viralityScore,omitempty"`
}
