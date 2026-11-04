package review

type QualitySummary struct {
	Reused bool   `json:"reused"`
	Status string `json:"status"`
}
type PRCommentsSummary struct {
	Read      bool        `json:"read"`
	Uncovered []PRComment `json:"uncovered"`
}
type ChunkSummary struct {
	Total  int `json:"total"`
	Failed int `json:"failed"`
}
type Result struct {
	Mode       string            `json:"mode"`
	Base       string            `json:"base"`
	Commit     string            `json:"commit"`
	Dirty      bool              `json:"dirty"`
	FailOn     string            `json:"failOn"`
	Exit       int               `json:"exit"`
	Quality    QualitySummary    `json:"quality"`
	Findings   []Finding         `json:"findings"`
	Discarded  []Discarded       `json:"discarded"`
	PRComments PRCommentsSummary `json:"prComments"`
	Chunks     ChunkSummary      `json:"chunks"`
	Summary    map[Severity]int  `json:"summary"`
	Warnings   []string          `json:"warnings,omitempty"`
	SavedAt    string            `json:"savedAt,omitempty"`
}

// Finalize gives empty arrays a stable JSON shape and computes the exit code.
// It may be called repeatedly after collecting partial results.
func (r *Result) Finalize() {
	if r.FailOn == "" {
		r.FailOn = "high"
	}
	r.Summary = map[Severity]int{Critical: 0, High: 0, Medium: 0, Low: 0}
	r.Exit = 0
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	if r.Discarded == nil {
		r.Discarded = []Discarded{}
	}
	if r.PRComments.Uncovered == nil {
		r.PRComments.Uncovered = []PRComment{}
	}
	for i := range r.Findings {
		f := &r.Findings[i]
		r.Summary[f.Severity]++
		if AtLeast(f.Severity, r.FailOn) {
			r.Exit = 1
		}
		if f.Passes == nil {
			f.Passes = []string{}
		}
		if f.ConfirmedBy == nil {
			f.ConfirmedBy = []string{}
		}
		if f.Severities == nil {
			f.Severities = []Severity{}
		}
		if f.PRComments == nil {
			f.PRComments = []PRComment{}
		}
	}
	if r.Chunks.Failed > 0 {
		r.Exit = 2
	}
}
