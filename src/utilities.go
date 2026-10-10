package main

import (
	"context"
	_ "embed"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/schollz/progressbar/v3"
)

//go:embed VERSION
var sourceVersion string

// Set by release builds; local builds use the checked-in source version.
var version string

const requestTimeout = 30 * time.Second

// Version returns the version of the CLI application
func (CLIArgs) Version() string {
	v := version
	if v == "" {
		v = strings.TrimSpace(sourceVersion)
	}
	return "url-finder " + v
}

// isURL checks if the given path is a valid URL
func isURL(path string) bool {
	parsedURL, err := url.ParseRequestURI(path)
	if err != nil {
		return false
	}
	// Check if the URL has a valid scheme and hostname
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}
	return parsedURL.Hostname() != ""
}

// isDirectory checks if the given path is a directory
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// isFile checks if the given path is a file
func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// extractURLs extracts all URLs from the given content
func extractURLs(content string) []string {
	// Stop matches at common URL terminators like whitespace, quotes, or angle brackets
	re := regexp.MustCompile(`https?://[^\s"'<>]+`)
	return re.FindAllString(content, -1)
}

// displayResults prints the results in a tabular format
func filterResults(results [][]string, noQuery bool, showAll bool) [][]string {
	if !showAll && !noQuery {
		var filtered [][]string
		for _, row := range results {
			// row format: [Type, Source, URL, IP, StatusCode, Redirect, ...]
			ip := row[3]
			// Keep DNS failures or HTTP lookup failures (no IP or no status)
			if ip == "" || row[4] == "" {
				filtered = append(filtered, row)
				continue
			}
			statusParts := strings.Split(row[4], " -> ")
			finalStatus := statusParts[len(statusParts)-1]
			// Keep any non-2xx status
			if !strings.HasPrefix(finalStatus, "2") {
				filtered = append(filtered, row)
			}
		}
		results = filtered
	}

	return results
}

// displayResults prints the actual filtered rows.
func displayResults(results [][]string, noQuery bool, showAll bool) {
	results = filterResults(results, noQuery, showAll)

	if noQuery {
		table := tablewriter.NewWriter(os.Stdout)
		headers := []string{"Type", "Source", "URL"}
		table.Header(headers)
		for _, row := range results {
			table.Append(row[:3])
		}
		table.Render()
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"Type", "Source", "URL", "IP Address", "Status Code"}
	table.Header(headers)

	// Append results to the table
	for _, result := range results {
		table.Append(result[:5])
	}

	table.Render()
}

// getURLStatus retrieves the IP address, status code chain, potential redirect location
// for the given URL.
func getURLStatus(rawURL string) (string, string, string) {
	return getURLStatusWithClient(rawURL, &http.Client{Timeout: requestTimeout})
}

func getURLStatusWithClient(rawURL string, baseClient *http.Client) (string, string, string) {
	if !isURL(rawURL) {
		return "", "", ""
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", "", ""
	}

	// Lookup IP address
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", parsedURL.Hostname())
	if err != nil || len(ips) == 0 {
		return "", "", ""
	}
	ip := ips[0].String()

	// Keep track of all intermediate status codes in case of redirects
	var statusChain []string

	client := *baseClient
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// The new request carries the response that caused this redirect.
		if req.Response != nil && len(via) < 2 {
			statusChain = append(statusChain, fmt.Sprintf("%d", req.Response.StatusCode))
		}
		if len(via) >= 2 {
			return http.ErrUseLastResponse
		}
		return nil
	}

	// #nosec G704 -- Probing user-selected URLs is the purpose of this CLI.
	resp, err := client.Get(rawURL)
	if err != nil {
		return ip, "", ""
	}
	defer resp.Body.Close()

	// Append the final response code
	finalCode := fmt.Sprintf("%d", resp.StatusCode)
	statusChain = append(statusChain, finalCode)

	// If it's an actual redirect (3xx), capture the 'Location' header
	redirectURL := ""
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		redirectURL = resp.Header.Get("Location")
	}

	// Join the chain of statuses with " -> "
	// e.g. "301 -> 200"
	fullStatus := strings.Join(statusChain, " -> ")

	return ip, fullStatus, redirectURL
}

// accessURLs processes the extracted URLs
func accessURLs(results [][]string, concurrency int) [][]string {
	if concurrency < 1 {
		concurrency = 1
	}
	var extractedURLs []string
	for _, result := range results {
		extractedURLs = append(extractedURLs, result[2])
	}

	if len(extractedURLs) == 0 {
		return results
	}

	urlChan := make(chan []string, len(extractedURLs))
	resultChan := make(chan []string, len(extractedURLs))

	var wg sync.WaitGroup

	// Progress bar for URL processing
	urlBar := progressbar.Default(int64(len(extractedURLs)), "Checking URLs")

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for result := range urlChan {
				ip, status, redirect := getURLStatus(result[2])
				result[3] = ip
				result[4] = status
				result[5] = redirect
				resultChan <- result
				urlBar.Add(1)
			}
		}()
	}

	for _, url := range results {
		urlChan <- url
	}
	close(urlChan)

	wg.Wait()
	close(resultChan)

	urlBar.Finish() // Ensure progress bar completes

	var finalResults [][]string
	for result := range resultChan {
		finalResults = append(finalResults, result)
	}

	return finalResults
}
