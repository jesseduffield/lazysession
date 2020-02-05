package utils

import (
	"errors"
	"strings"
)

// RenderStringsWithPadding takes an array of rows each containing an array of columns, to be displayed with padding in a tabular form
func RenderStringsWithPadding(stringArrays [][]string) (string, error) {
	if !displayArraysAligned(stringArrays) {
		return "", errors.New("Each item must return the same number of strings to display")
	}

	padWidths := getPadWidths(stringArrays)
	paddedDisplayStrings := getPaddedDisplayStrings(stringArrays, padWidths)

	return strings.Join(paddedDisplayStrings, "\n"), nil
}

func getPadWidths(stringArrays [][]string) []int {
	if len(stringArrays[0]) <= 1 {
		return []int{}
	}
	padWidths := make([]int, len(stringArrays[0])-1)
	for i := range padWidths {
		for _, strings := range stringArrays {
			uncoloredString := Decolorise(strings[i])
			if len(uncoloredString) > padWidths[i] {
				padWidths[i] = len(uncoloredString)
			}
		}
	}
	return padWidths
}

func getPaddedDisplayStrings(stringArrays [][]string, padWidths []int) []string {
	paddedDisplayStrings := make([]string, len(stringArrays))
	for i, stringArray := range stringArrays {
		if len(stringArray) == 0 {
			continue
		}
		for j, padWidth := range padWidths {
			paddedDisplayStrings[i] += WithPadding(stringArray[j], padWidth) + " "
		}
		paddedDisplayStrings[i] += stringArray[len(padWidths)]
	}
	return paddedDisplayStrings
}

// displayArraysAligned returns true if every string array returned from our
// list of displayables has the same length
func displayArraysAligned(stringArrays [][]string) bool {
	for _, strings := range stringArrays {
		if len(strings) != len(stringArrays[0]) {
			return false
		}
	}
	return true
}

// WithPadding pads a string as much as you want
func WithPadding(str string, padding int) string {
	uncoloredStr := Decolorise(str)
	if padding < len(uncoloredStr) {
		return str
	}
	return str + strings.Repeat(" ", padding-len(uncoloredStr))
}
