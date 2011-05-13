package main

import (
	"fmt"
	"flag"
)

func printVersion(pkg string, version string, author string) {
	fmt.Println(pkg, version)
	fmt.Println("Copyright (C) 2010, 2011", author)
	fmt.Println("This program is free software; you may redistribute it under the terms of")
	fmt.Println("the GNU General Public License version 3 or (at your option) a later version.")
	fmt.Println("This program has absolutely no warranty.")
	fmt.Println("Report bugs to bug@freeconsole.org")
}

func printHelp(pkg string, version string, about string, usage string) {
	fmt.Println(pkg, version, "\n")
	fmt.Println(about)
	fmt.Println("Usage:")
	fmt.Println("\t", usage)
	fmt.Println("Options:")
	flag.PrintDefaults()
}
