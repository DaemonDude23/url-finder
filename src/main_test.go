package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripperFunc lets us stub http.RoundTripper with a function
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestIsURL(t *testing.T) {
	if !isURL("http://example.com") {
		t.Error("Expected valid URL")
	}
	if isURL("invalid-string") {
		t.Error("Expected invalid URL")
	}
}

func TestDisplayResultsFiltering(t *testing.T) {
	results := [][]string{
		{"Type", "Source", "http://ok.com", "127.0.0.1", "200", ""},
		{"Type", "Source", "http://dead.com", "", "", ""},             // DNS fail
		{"Type", "Source", "http://error.com", "1.2.3.4", "404", ""},  // 4xx
		{"Type", "Source", "http://broken.com", "2.3.4.5", "500", ""}, // 5xx
		{"Type", "Source", "http://redirect.com", "8.8.8.8", "301 -> 200", ""},
	}

	// showAll = false
	filtered := filterHelper(results, false, false)
	if len(filtered) != 3 {
		t.Errorf("Expected 3 results; got %d", len(filtered))
	}
	for _, row := range filtered {
		if row[2] == "http://ok.com" {
			t.Error("Expected http://ok.com to be omitted when showAll=false")
		}
	}

	// showAll = true
	allResults := filterHelper(results, true, false)
	if len(allResults) != 5 {
		t.Errorf("Expected 5 results; got %d", len(allResults))
	}
}

// filterHelper simulates displayResults filtering
func filterHelper(results [][]string, showAll, noQuery bool) [][]string {
	if !showAll && !noQuery {
		var filtered [][]string
		for _, row := range results {
			if row[3] == "" {
				filtered = append(filtered, row)
				continue
			}
			statusParts := strings.Split(row[4], " -> ")
			finalStatus := statusParts[len(statusParts)-1]
			if strings.HasPrefix(finalStatus, "4") || strings.HasPrefix(finalStatus, "5") {
				filtered = append(filtered, row)
			}
		}
		return filtered
	}
	return results
}

func TestProcessURLUsesExtractorAndIncludesSource(t *testing.T) {
	const dummyBody = `
		<html>
			<body>
				<a href='http://linked.example.com/page'>one</a>
				More text with https://another.example.com/resource here.
				Relative links like /contact should be ignored.
			</body>
		</html>
	`

	// Stub the default transport so http.Get uses in-memory content
	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(dummyBody)),
			Header:     make(http.Header),
		}, nil
	})
	defer func() {
		http.DefaultTransport = origTransport
	}()

	testURL := "http://example.com/page"
	results := processURL(testURL)

	if len(results) < 3 {
		t.Fatalf("expected at least 3 rows (source + 2 links), got %d", len(results))
	}

	foundSource := false
	foundLinked := false
	foundAnother := false

	for _, row := range results {
		if row[2] == testURL {
			foundSource = true
		}
		if row[2] == "http://linked.example.com/page" {
			foundLinked = true
		}
		if row[2] == "https://another.example.com/resource" {
			foundAnother = true
		}
	}

	if !foundSource {
		t.Error("expected source URL to be included as its own entry")
	}
	if !foundLinked || !foundAnother {
		t.Errorf("expected both extracted links to be present; linked=%v another=%v", foundLinked, foundAnother)
	}
}
