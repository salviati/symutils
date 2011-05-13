package main

import (
	"fmt"
	"os"
	"log"
	"strings"
)

/* Presents a yes/no question. Returns 1 if yes, 0 otherwise. */
func ynQuestion(format string, va ...interface{}) bool {
	if *yesToAll {
		return true
	}

	for {
		fmt.Fprintf(os.Stderr, format, va...)
		fmt.Fprintf(os.Stderr, " (y/n): ")
		r := ""
		_, err := fmt.Scanf("%s", &r)
		if err != nil {
			log.Fatal(err)
		}
		r = strings.TrimSpace(r)

		switch r {
		case "y", "Y":
			return true
		case "n", "N":
			return false
		default:
			fmt.Fprintf(os.Stderr, "hint: say y or n :)\n")
		}
	}
	return false
}

/* Presents the possible targets and reads the user choice until we have a valid choice
   or user explicitly cancels.
   Returns -1 if user cancels, target index otherwise. */
func getInteractiveChoice(sl []string) (choice int) {
	for i, s := range sl {
		fmt.Fprintf(os.Stderr, "[%d] %s\n", i, s)
	}

	for {
		fmt.Fprintf(os.Stderr, "* Which one seems to be the correct target? (leave blank to skip) [range %d-%d]: ", 0, len(sl)-1)
		choice = 0
		_, err := fmt.Scanf("%d", &choice)
		if err != nil {
			return -1
		}

		if err == nil && choice >= 0 && choice < len(sl) {
			break
		}

		fmt.Fprintf(os.Stderr, "* %v is not a valid choice, let's try again\n", choice)
	}
	return
}
