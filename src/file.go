package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/schollz/progressbar/v3"
)

// processDirectory processes all files in the given directory using allowed file extensions.
func processDirectory(dir string, fileExtensions string) [][]string {
	// Parse allowed extensions into a map for O(1) lookup
	extsSlice := strings.Split(fileExtensions, ",")
	allowedExts := make(map[string]bool, len(extsSlice))
	for _, ext := range extsSlice {
		allowedExts[strings.ToLower(strings.TrimSpace(ext))] = true
	}

	var files []string
	var results [][]string

	// Walk through the directory and collect file paths matching allowed extensions
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			if allowedExts[strings.ToLower(filepath.Ext(path))] {
				files = append(files, path)
			}
		}
		return nil
	})
	if err != nil {
		log.Info("Error processing directory:", err)
		return results
	}

	// Only create progress bar if there are files to process
	if len(files) > 0 {
		// Progress bar for file processing
		fileBar := progressbar.Default(int64(len(files)), "Reading Files")

		// Process each file and extract URLs
		for _, file := range files {
			results = append(results, processFile(file)...)
			fileBar.Add(1)
		}

		fileBar.Finish() // Ensure progress bar completes
	}

	return results
}

// processFile processes a single file and extracts URLs
func processFile(filePath string) [][]string {
	var results [][]string

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		log.Info("Error opening file:", err)
		return results
	}
	defer file.Close()

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		urls := extractURLs(line)
		if len(urls) > 0 {
			for _, u := range urls {
				// Append the line number to the results
				results = append(results, []string{"File", filePath, u, "", "", "", "", strconv.Itoa(lineNumber)})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Info("Error reading file:", err)
	}

	return results
}
