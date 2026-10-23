package quality

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
)

type TestCounts struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type trxTestRun struct {
	XMLName       xml.Name         `xml:"TestRun"`
	ResultSummary trxResultSummary `xml:"ResultSummary"`
}

type trxResultSummary struct {
	Counters trxCounters `xml:"Counters"`
}

type trxCounters struct {
	Total       int `xml:"total,attr"`
	Passed      int `xml:"passed,attr"`
	Failed      int `xml:"failed,attr"`
	NotExecuted int `xml:"notExecuted,attr"`
}

// ParseTRX sums <ResultSummary><Counters total passed failed …> of every *.trx in dir (notExecuted counts as Skipped).
func ParseTRX(dir string) (*TestCounts, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.trx"))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}

	var counts TestCounts
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			return nil, err
		}
		var run trxTestRun
		if err := xml.Unmarshal(data, &run); err != nil {
			return nil, err
		}
		c := run.ResultSummary.Counters
		counts.Total += c.Total
		counts.Passed += c.Passed
		counts.Failed += c.Failed
		counts.Skipped += c.NotExecuted
	}

	return &counts, nil
}

type jestReport struct {
	NumTotalTests   int `json:"numTotalTests"`
	NumPassedTests  int `json:"numPassedTests"`
	NumFailedTests  int `json:"numFailedTests"`
	NumPendingTests int `json:"numPendingTests"`
	NumTodoTests    int `json:"numTodoTests"`
}

// ParseJestJSON reads numTotalTests/numPassedTests/numFailedTests/numPendingTests (+numTodoTests in Skipped); serves for jest and vitest.
func ParseJestJSON(path string) (*TestCounts, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r jestReport
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &TestCounts{
		Total:   r.NumTotalTests,
		Passed:  r.NumPassedTests,
		Failed:  r.NumFailedTests,
		Skipped: r.NumPendingTests + r.NumTodoTests,
	}, nil
}

// ParseReport dispatches to ParseTRX or ParseJestJSON depending on provider.
func ParseReport(provider, report string) (*TestCounts, error) {
	if report == "" {
		return nil, nil
	}
	switch provider {
	case "dotnet-test":
		return ParseTRX(report)
	case "vitest", "jest":
		return ParseJestJSON(report)
	default:
		return nil, nil
	}
}
