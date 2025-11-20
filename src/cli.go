package main

import (
	"strings"

	log "github.com/sirupsen/logrus"
)

// Contains CLI arguments
type CLIArgs struct {
	// CLI args only
	LogFormat string `arg:"--log-format" default:"plain" help:"Set log format: plain or JSON"`
	LogLevel  string `arg:"-l, --log-level" default:"INFO" help:"Set log level: INFO, DEBUG, ERROR, WARNING"`
	LogColors bool   `arg:"--log-colors" default:"true" help:"Enables color in the log output"`

	Columns        string `arg:"--columns" default:"type,URL,IPAddress,StatusCode,ResponseTime" help:"Columns to display in the output"`
	FileExtensions string `arg:"--file-extensions" default:".md,.markdown,.txt,.text,.log,.cfg,.conf,.ini,.json,.yaml,.yml,.xml,.html,.htm" help:"Comma-separated list of file extensions to parse"`
	NoQuery        bool   `arg:"--no-query" help:"Disable querying URLs"`
	QueryThreads   int    `arg:"--query-threads" default:"3" help:"Number of concurrent threads to use for querying URLs"`
	ShowAll        bool   `arg:"--show-all" default:"false" help:"Show all results, or only failed URLs"`
	Target         string `arg:"positional, required" help:"Path to url-finder config file or URL"`
	UserAgent      string `arg:"--user-agent" default:"url-finder/1.0" help:"Set the User-Agent header for the HTTP request"`
}

// Checks the CLI arguments and sets up logging
func SetupLogging(args CLIArgs) bool {
	// Setup logging defaults
	log.SetLevel(log.InfoLevel)

	// Set Log level from args
	if strings.EqualFold(args.LogLevel, "TRACE") {
		log.SetReportCaller(true)
		log.SetLevel(log.TraceLevel)
		log.Debugf("Log level set to TRACE")
	} else if strings.EqualFold(args.LogLevel, "DEBUG") {
		log.SetLevel(log.DebugLevel)
		log.Debugf("Log level set to DEBUG")
	} else if strings.EqualFold(args.LogLevel, "ERROR") {
		log.SetLevel(log.ErrorLevel)
		log.Debugf("Log level set to ERROR")
	} else if strings.EqualFold(args.LogLevel, "WARNING") {
		log.SetLevel(log.WarnLevel)
		log.Debugf("Log level set to WARNING")
	} else {
		log.SetLevel(log.InfoLevel)
	}

	// Log format
	if strings.EqualFold(args.LogFormat, "JSON") {
		log.SetFormatter(&log.JSONFormatter{})
		log.Debugf("Log format set to JSON")
	} else if !strings.EqualFold(args.LogLevel, "TRACE") && !strings.EqualFold(args.LogLevel, "DEBUG") {
		log.SetFormatter(&log.TextFormatter{ForceColors: args.LogColors, FullTimestamp: false})
		log.Debugf("Log format set to stylized TextFormatter (default)")
	}

	return true
}
