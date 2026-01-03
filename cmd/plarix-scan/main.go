// Package main implements the plarix-scan CLI.
//
// Purpose: Entry point for the Plarix Scan GitHub Action CLI.
// Public API: `run` subcommand with --command flag.
// Usage: plarix-scan run --command "pytest -q"
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// Version is set from VERSION file at build time or read at runtime.
const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("plarix-scan v%s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// printUsage prints CLI usage information.
func printUsage() {
	fmt.Println(`Usage: plarix-scan <command> [options]

Commands:
  run       Run a command with LLM API cost tracking
  version   Print version information
  help      Show this help message

Run Options:
  --command <string>   Command to execute (required)
  --pricing <path>     Path to custom pricing JSON
  --fail-on-cost <float>   Exit non-zero if cost exceeds threshold (USD)
  --providers <csv>    Providers to intercept (default: openai,anthropic,openrouter)
  --comment <mode>     Comment mode: pr, summary, both (default: both)
  --enable-openai-stream-usage-injection <bool>   Opt-in for OpenAI stream usage (default: false)`)
}

// runCmd handles the "run" subcommand.
func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)

	command := fs.String("command", "", "Command to execute (required)")
	_ = fs.String("pricing", "", "Path to custom pricing JSON")
	_ = fs.Float64("fail-on-cost", 0, "Exit non-zero if cost exceeds threshold (USD)")
	_ = fs.String("providers", "openai,anthropic,openrouter", "Providers to intercept")
	_ = fs.String("comment", "both", "Comment mode: pr, summary, both")
	_ = fs.Bool("enable-openai-stream-usage-injection", false, "Opt-in for OpenAI stream usage")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if *command == "" {
		// Try environment variable (set by action.yml)
		if envCmd := os.Getenv("INPUT_COMMAND"); envCmd != "" {
			*command = envCmd
		} else {
			fmt.Fprintln(os.Stderr, "Error: --command is required")
			os.Exit(1)
		}
	}

	// For v0.1.0: Just write Step Summary and exit
	// Future milestones will add proxy, command execution, and cost tracking
	summary := fmt.Sprintf("## ✅ Plarix Scan Installed (v%s)\n\n", version)
	summary += fmt.Sprintf("Command configured: `%s`\n\n", *command)
	summary += "**Note:** This is the initial stub. Proxy and cost tracking coming in v0.2.0+\n"

	writeStepSummary(summary)
	fmt.Println(summary)
}

// writeStepSummary writes content to GitHub Step Summary if available.
func writeStepSummary(content string) {
	summaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryPath == "" {
		return
	}

	// Append to summary file
	f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write step summary: %v\n", err)
		return
	}
	defer f.Close()

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	f.WriteString(content)
}
