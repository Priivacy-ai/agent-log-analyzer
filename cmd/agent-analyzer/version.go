package main

import "fmt"

const sourceURL = "https://github.com/robertDouglass/claude-log-analyzer"

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func printVersion() {
	fmt.Printf("agent-analyzer %s\n", version)
	fmt.Printf("commit: %s\n", commit)
	fmt.Printf("built: %s\n", date)
	fmt.Printf("source: %s\n", sourceURL)
}
