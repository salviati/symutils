package common

import (
	"fmt"
	"os"
)

type LogLevelType int

var (
	ShowLogLevel = true
	ErrorsFatal  = true
	LogLevel     = LogLevelType(0)
)

var (
	levelString  = []string{"[ERR] ", "[WRN] ", "[LOG] "}
	unknownLevel = "[???] "
)

func (l LogLevelType) String() string {
	if l < 0 || int(l) >= len(levelString) {
		return unknownLevel
	}
	s := levelString[l]
	return s
}

const ( //FIXME imp. a type w/ String() instead
	ERR = iota
	WARN
	LOG
)

const (
	LogLevelMin = 0
	LogLevelMax = 2
)

func SetLogLevel(l uint) {
	if l > LogLevelMax {
		l = LogLevelMax
	}
	LogLevel = LogLevelType(int(l))
}

/* Verbose print function.
 * Prints out given message on a given level (with proper suffix if ShowLogLevel is set)
 * If level is ERR, exits the program with error code 1. */
func Printf(level LogLevelType, format string, a ...interface{}) {
	if level == ERR && ErrorsFatal {
		defer os.Exit(1)
	}
	if LogLevel < level {
		return
	}
	if ShowLogLevel {
		fmt.Fprint(os.Stderr, level)
	}

	fmt.Fprintf(os.Stderr, format, a...)
}

func Print(level LogLevelType, a ...interface{}) {
	if level == ERR && ErrorsFatal {
		defer os.Exit(1)
	}
	if LogLevel < level {
		return
	}
	if ShowLogLevel {
		fmt.Fprint(os.Stderr, level)
	}

	fmt.Fprint(os.Stderr, a...)
}

func Println(level LogLevelType, a ...interface{}) {
	if level == ERR && ErrorsFatal {
		defer os.Exit(1)
	}
	if LogLevel < level {
		return
	}
	if ShowLogLevel {
		fmt.Fprint(os.Stderr, level)
	}

	fmt.Fprintln(os.Stderr, a...)
}

func Logf(format string, a ...interface{}) {
	Printf(LOG, format, a...)
}

func Log(a ...interface{}) {
	Print(LOG, a...)
}

func Logln(a ...interface{}) {
	Println(LOG, a...)
}

func Warnf(format string, a ...interface{}) {
	Printf(WARN, format, a...)
}

func Warn(a ...interface{}) {
	Print(WARN, a...)
}

func Warnln(a ...interface{}) {
	Println(WARN, a...)
}

func Errorf(format string, a ...interface{}) {
	Printf(ERR, format, a...)
}

func Error(a ...interface{}) {
	Print(ERR, a...)
}

func Errorln(a ...interface{}) {
	Println(ERR, a...)
}
