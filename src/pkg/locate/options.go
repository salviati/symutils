package locate

import (
	"symutils/fuzzy"
)

// Database search options.

type Options struct {
	IgnoreCase           bool
	StripExtension       bool   // Ignore extensions when searching.
	Basename             bool   // Basename _must_ match
	StripPath            bool   // Match only basenames, ignoring directory parts.
	Existing             bool   // List only existing files.
	Accessable           bool   // List only (read-) accessable files
	Symlink              bool   // List symlinks as well.
	HashMap              bool   // Enable this if you want to the lookup table created by NewDB.
	IgnoreChars          string // List of characters to be filtered out in target names.
	LevenshteinCost      fuzzy.LevenshteinCost // Coefficients for the Levenshtein distance.
	LevenshteinThreshold int // Threshold value for Levenshtein cost.
	MaxMatches           uint // Search will halt when the number of hits reaches to this number.
	NWorkers             uint // Number of goroutines for searching in a single database.
}
