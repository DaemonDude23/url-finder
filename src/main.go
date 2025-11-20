package main

import (
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/alexflint/go-arg"
)

func main() {
	// Parse CLI arguments
	var args CLIArgs
	arg.MustParse(&args)
	SetupLogging(args)

	decisionLogic(args)

	os.Exit(0)
}

func decisionLogic(args CLIArgs) {
	// Check if the path is a directory, file, or URL
	var results [][]string
	if isURL(args.Target) {
		log.Debug("It's a URL: ", args.Target)
		results = processURL(args.Target)
	} else if isDirectory(args.Target) {
		log.Debug("It's a directory: ", args.Target)
		results = processDirectory(args.Target, args.FileExtensions)
	} else if isFile(args.Target) {
		log.Debug("It's a file: ", args.Target)
		results = processFile(args.Target)
	} else {
		log.Error("Invalid path: ", args.Target)
		return
	}

	if !args.NoQuery {
		results = accessURLs(results, args.QueryThreads)
	}

	displayResults(results, args.NoQuery, args.ShowAll)
}
