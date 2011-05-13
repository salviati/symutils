package main

import (
	"os"
	"fmt"
	"flag"
)

var verbose = flag.Int("v", 0, "Set verbosity level (0-3)")
var showLogLevel = flag.Bool("L", false, "Show log level for messages")

const ( //FIXME imp. a type w/ String() instead
	ERR  = iota
	WARN = iota
	INFO = iota
	LOG  = iota
)

var verbosityLevel = []string{"[ERR ]", "[WARN]", "[INFO]", "[LOG ]"}
/* Verbose print function.
 * Prints out given message on a given level (with proper suffix if ShowLogLevel is set)
 * If level is ERR, exits the program with error code 1. */
func vprintf(level int, format string, a ...interface{}) {
	if level == ERR {
		defer os.Exit(1)
	} //FIXME

	if *verbose < level {
		return
	}

	if *showLogLevel {
		fmt.Fprintf(os.Stderr, "%s ", verbosityLevel[level])
	}
	fmt.Fprintf(os.Stderr, format, a...)
}

func vprintln(level int, a ...interface{}) {
	if level == ERR {
		defer os.Exit(1)
	} //FIXME

	if *verbose < level {
		return
	}

	if *showLogLevel {
		fmt.Fprint(os.Stderr, verbosityLevel[level])
	}
	fmt.Fprintln(os.Stderr, a...)
}
