package main

import (
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

func processURL(target string) [][]string {
	var results [][]string

	resp, err := http.Get(target)
	if err != nil {
		log.Info("Error fetching URL:", err)
		return results
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Info("Error reading response body:", err)
		return results
	}

	bodyString := string(body)
	// Always include the starting page itself
	results = append(results, []string{"URL", target, target, "", "", "", "", ""})

	for _, link := range extractURLs(bodyString) {
		results = append(results, []string{"URL", target, strings.TrimSpace(link), "", "", "", "", ""})
	}

	return results
}
