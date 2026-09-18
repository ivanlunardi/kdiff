package differ

import (
	"fmt"
	"os"
	"strings"
)

// SplitLines splits a text into lines, handling both \r\n and \n endings.
func SplitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	// Normalize \r\n to \n
	normalized := strings.ReplaceAll(s, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	// If ends with newline, trim the last empty element if trailing newline
	lines := strings.Split(normalized, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// DiffLines performs Myers diff algorithm on two string contents.
// Returns list of DiffLine, additions count, deletions count.
func DiffLines(leftContent, rightContent string) ([]DiffLine, int, int) {
	leftLines := SplitLines(leftContent)
	rightLines := SplitLines(rightContent)

	return ComputeDiff(leftLines, rightLines)
}

// ComputeDiff executes Myers diff algorithm on two string slices.
func ComputeDiff(a, b []string) ([]DiffLine, int, int) {
	n := len(a)
	m := len(b)

	if n == 0 && m == 0 {
		return []DiffLine{}, 0, 0
	}

	if n == 0 {
		lines := make([]DiffLine, m)
		for i, line := range b {
			lines[i] = DiffLine{
				LeftLineNum:  0,
				RightLineNum: i + 1,
				Type:         LineInsert,
				Content:      line,
			}
		}
		return lines, m, 0
	}

	if m == 0 {
		lines := make([]DiffLine, n)
		for i, line := range a {
			lines[i] = DiffLine{
				LeftLineNum:  i + 1,
				RightLineNum: 0,
				Type:         LineDelete,
				Content:      line,
			}
		}
		return lines, 0, n
	}

	max := n + m
	v := make([]int, 2*max+1)
	history := make([][]int, 0, max+1)

	v[max+1] = 0

	var dFound = -1
	for d := 0; d <= max; d++ {
		vCopy := make([]int, len(v))
		copy(vCopy, v)
		history = append(history, vCopy)

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[max+k-1] < v[max+k+1]) {
				x = v[max+k+1]
			} else {
				x = v[max+k-1] + 1
			}
			y := x - k

			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[max+k] = x

			if x >= n && y >= m {
				dFound = d
				break
			}
		}

		if dFound != -1 {
			break
		}
	}

	// Backtracking
	var result []DiffLine
	x := n
	y := m

	for d := dFound; d > 0; d-- {
		k := x - y
		vPrev := history[d]

		var prevK int
		if k == -d || (k != d && vPrev[max+k-1] < vPrev[max+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}

		prevX := vPrev[max+prevK]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			x--
			y--
			result = append(result, DiffLine{
				LeftLineNum:  x + 1,
				RightLineNum: y + 1,
				Type:         LineEqual,
				Content:      a[x],
			})
		}

		if x == prevX {
			y--
			result = append(result, DiffLine{
				LeftLineNum:  0,
				RightLineNum: y + 1,
				Type:         LineInsert,
				Content:      b[y],
			})
		} else if y == prevY {
			x--
			result = append(result, DiffLine{
				LeftLineNum:  x + 1,
				RightLineNum: 0,
				Type:         LineDelete,
				Content:      a[x],
			})
		}
	}

	for x > 0 && y > 0 {
		x--
		y--
		result = append(result, DiffLine{
			LeftLineNum:  x + 1,
			RightLineNum: y + 1,
			Type:         LineEqual,
			Content:      a[x],
		})
	}

	// Reverse result
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	additions := 0
	deletions := 0
	for _, line := range result {
		switch line.Type {
		case LineInsert:
			additions++
		case LineDelete:
			deletions++
		}
	}

	return result, additions, deletions
}

// ScannedInput encapsulates the data required to diff a single file.
type ScannedInput struct {
	RelativePath string
	LeftPath     string
	RightPath    string
	Status       FileStatus
	IsBinary     bool
	LeftSize     int64
	RightSize    int64
}

// DiffFile produces a FileDiff for a given scanned file input.
func DiffFile(input ScannedInput) (FileDiff, error) {
	fd := FileDiff{
		RelativePath: input.RelativePath,
		Status:       input.Status,
		IsBinary:     input.IsBinary,
		LeftSize:     input.LeftSize,
		RightSize:    input.RightSize,
		Lines:        []DiffLine{},
	}

	if input.IsBinary {
		return fd, nil
	}

	var leftContent, rightContent string
	if input.LeftPath != "" {
		data, err := os.ReadFile(input.LeftPath)
		if err != nil {
			return fd, fmt.Errorf("reading left file %s: %w", input.LeftPath, err)
		}
		leftContent = string(data)
	}

	if input.RightPath != "" {
		data, err := os.ReadFile(input.RightPath)
		if err != nil {
			return fd, fmt.Errorf("reading right file %s: %w", input.RightPath, err)
		}
		rightContent = string(data)
	}

	lines, additions, deletions := DiffLines(leftContent, rightContent)
	fd.Lines = lines
	fd.Additions = additions
	fd.Deletions = deletions

	return fd, nil
}
