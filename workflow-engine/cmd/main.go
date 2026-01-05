// Package main provides the entry point for the workflow-engine CLI.
package main

import (
	"fmt"
	"os"

	"yqhp/workflow-engine/cmd/master"
	"yqhp/workflow-engine/cmd/run"
	"yqhp/workflow-engine/cmd/slave"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	var err error
	switch command {
	case "master":
		err = master.Execute(args)
	case "slave":
		err = slave.Execute(args)
	case "run":
		err = run.Execute(args)
	case "version":
		fmt.Printf("workflow-engine version %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`workflow-engine - A distributed workflow execution engine

Usage:
  workflow-engine <command> [options]

Commands:
  master    Manage the master node
  slave     Manage slave nodes
  run       Execute a workflow in standalone mode
  version   Print version information
  help      Show this help message

Use "workflow-engine <command> --help" for more information about a command.`)
}
