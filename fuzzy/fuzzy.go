/*
   Copyright (c) Utkan Güngördü <utkan@freeconsole.org>

   This program is free software; you can redistribute it and/or modify
   it under the terms of the GNU General Public License as
   published by the Free Software Foundation; either version 3 or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of

   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the

   GNU General Public License for more details


   You should have received a copy of the GNU General Public
   License along with this program; if not, write to the
   Free Software Foundation, Inc.,
   51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
*/

// This package implements some fuzzy search algorithms.
package fuzzy

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func min3(a, b, c int) int {
	return min(min(a, b), c)
}

// Costs for deletion, insertion and subtition.
type LevenshteinCost struct {
	Del, Ins, Subs int
}


// LevenshteinDistance makes a fuzzy search, using Levenshtein distance as a measure,
// for needle in haystack.
// Runs at O(m*n), when m and n are length of needle and haystack.
// Uses O(n) memory.
func Levenshtein(needle string, haystack string, cost *LevenshteinCost) int {
	if len(needle) == 0 {
		return len(haystack) * cost.Ins
	}
	if len(haystack) == 0 {
		return len(needle) * cost.Del
	}

	d := make([]int, len(haystack)+1)
	e := make([]int, len(haystack)+1)

	for i := 0; i < len(needle); i++ {
		e[0] = d[0] + 1
		for j := 0; j < len(haystack); j++ {
			c := 0
			if needle[i] != haystack[j] {
				c = cost.Subs
			}
			e[j+1] = min3(d[j+1]+cost.Del, e[j]+cost.Ins, d[j]+c)
		}
		d, e = e, d // We don't need contents of d anymore, so "move" e to d,
	}
	return d[len(d)-1]
}
