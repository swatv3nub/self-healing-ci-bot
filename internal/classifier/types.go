package classifier

// ClassificationResult represents the output of the failure classification process
type ClassificationResult struct {
	Category    FailureCategory `json:"category"`
	Confidence  float64         `json:"confidence"` // 0.0 to 1.0
	Evidence    []string        `json:"evidence"`   // Lines from logs supporting the classification
	CulpritLine *CulpritLine    `json:"culprit_line,omitempty"`
	Reasoning   string          `json:"reasoning"`
	Method      string          `json:"method"` // "heuristic" or "llm"
}

// FailureCategory represents the type of failure
type FailureCategory string

const (
	CategoryInfra FailureCategory = "infra"
	CategoryFlaky FailureCategory = "flaky"
	CategoryReal  FailureCategory = "real"
)

// CulpritLine represents the specific line in the code/logs that likely caused the failure
type CulpritLine struct {
	File     string `json:"file"`
	LineNo   int    `json:"line_no"`
	Function string `json:"function,omitempty"`
	Content  string `json:"content"`
}

// FailureLogInput is what we pass to the classifier
type FailureLogInput struct {
	Provider     string // "github", "gitlab", etc.
	JobName      string
	TestName     string // if applicable
	Logs         string // raw job/test output
	Metadata     map[string]string
	PreviousRuns int // number of times this test failed recently
}
