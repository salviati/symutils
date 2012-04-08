package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Rule struct {
	method string
	s      string // Source target filename, or the pattern to match the source target filename
	d      string // New target for the symlink
	re     *regexp.Regexp
}

func trim(str string) string {
	return string(bytes.TrimSpace([]byte(str)))
}

// Parses a line of rule of the format:
//     [rule] `[src]` `[dst]`
// Where rule is a single character determining the match method: h(ashmap), s(ubstring), w(ildcard), r(egexp).
// If [rule] is skipped an the line starts which a ` character directly, then it's assumed to be h(ashmap).
func NewRule(line string) (*Rule, error) {
	r := new(Rule)

	if line[0] == '`' {
		r.method = "hashmap"
	} else {
		switch line[0] {
		case 's':
			r.method = "substring"
		case 'w':
			r.method = "wildcard"
		case 'r':
			r.method = "regexp"
		default:
			return nil, errors.New("Unknown method: " + line[0:1])
		}
		line = line[1:]
	}

	nbackslash := 0
	for _, c := range line {
		if c == '`' {
			nbackslash++
		}
	}
	if nbackslash != 4 {
		return nil, errors.New("Invalid input: " + line)
	}

	// 	line
	pos := 0

	next := func() (str string) {
		for _, c := range line[pos:] {
			pos++
			if c == '`' {
				break
			}
		}

		for _, c := range line[pos:] {
			pos++
			if c == '`' {
				break
			}
			str += string(c)
		}
		return
	}
	r.s = next()
	r.d = next()

	if r.s == "" || r.d == "" {
		return nil, errors.New("src and dst should not be empty strings")
	}

	if r.method == "regexp" {
		r.re = regexp.MustCompile(r.s)
	}

	return r, nil
}

func (r *Rule) String() string {
	return r.method + ": " + r.s + " -> " + r.d
}

func (r *Rule) Match(pattern string) bool {
	switch r.method {
	case "hashmap":
		return pattern == r.s
	case "substring":
		return strings.Contains(pattern, r.s)
	case "wildcard":
		m, _ := filepath.Match(pattern, r.s)
		return m
	case "regexp":
		return r.re.MatchString(r.s)
	}
	panic("shouldn't happen")
}

type Replacer struct {
	rules []*Rule
}

func (replacer *Replacer) Add(line string) error {
	rule, err := NewRule(line)
	if err != nil {
		return err
	}
	replacer.rules = append(replacer.rules, rule)

	return nil
}

func NewReplacer(rulefile string) (replacer *Replacer, err error) {
	f, err := os.Open(rulefile)
	if err != nil {
		return
	}

	file, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}

	replacer = new(Replacer)

	buf := bytes.NewBuffer(file)
	for nline := 1; ; nline++ {
		line, err := buf.ReadString('\n') // BUG(utkan): Will not work in Windows.
		line = trim(line)

		if len(line) != 0 && line[0] != '#' {
			if err = replacer.Add(line); err != nil {
				return nil, fmt.Errorf("NewReplacer: invalid input in rule file %s:%d: %s\n", rulefile, nline, err)
			}
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}
	}

	return
}

func (r *Replacer) Replace(filename string) (matches []string) {
	matches = make([]string, 0)
	for _, rule := range r.rules {
		if rule.Match(filename) {
			matches = append(matches, rule.d)
		}
	}
	return
}
