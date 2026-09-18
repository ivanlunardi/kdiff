package differ_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ivanlunardi/kdiff/differ"
)

func TestComputeDiff(t *testing.T) {
	tests := []struct {
		name          string
		left          string
		right         string
		wantAdditions int
		wantDeletions int
	}{
		{
			name:          "Both empty",
			left:          "",
			right:         "",
			wantAdditions: 0,
			wantDeletions: 0,
		},
		{
			name:          "Identical single line",
			left:          "hello world\n",
			right:         "hello world\n",
			wantAdditions: 0,
			wantDeletions: 0,
		},
		{
			name:          "Left only (pure deletions)",
			left:          "line 1\nline 2\nline 3\n",
			right:         "",
			wantAdditions: 0,
			wantDeletions: 3,
		},
		{
			name:          "Right only (pure additions)",
			left:          "",
			right:         "line 1\nline 2\n",
			wantAdditions: 2,
			wantDeletions: 0,
		},
		{
			name:          "Mixed modifications and additions",
			left:          "apple\nbanana\ncherry\n",
			right:         "apple\nblueberry\ncherry\ndate\n",
			wantAdditions: 2, // blueberry, date
			wantDeletions: 1, // banana
		},
		{
			name:          "CRLF normalization",
			left:          "line 1\r\nline 2\r\n",
			right:         "line 1\nline 2\n",
			wantAdditions: 0,
			wantDeletions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, additions, deletions := differ.DiffLines(tt.left, tt.right)
			if additions != tt.wantAdditions {
				t.Errorf("DiffLines() additions = %d, want %d", additions, tt.wantAdditions)
			}
			if deletions != tt.wantDeletions {
				t.Errorf("DiffLines() deletions = %d, want %d", deletions, tt.wantDeletions)
			}

			// Validate line numbering integrity
			leftCount := 0
			rightCount := 0
			for _, l := range lines {
				switch l.Type {
				case differ.LineEqual:
					leftCount++
					rightCount++
					if l.LeftLineNum != leftCount || l.RightLineNum != rightCount {
						t.Errorf("line %v numbering mismatch: got left %d, right %d; want %d, %d", l, l.LeftLineNum, l.RightLineNum, leftCount, rightCount)
					}
				case differ.LineDelete:
					leftCount++
					if l.LeftLineNum != leftCount || l.RightLineNum != 0 {
						t.Errorf("deleted line %v numbering mismatch: got left %d, right %d; want %d, 0", l, l.LeftLineNum, l.RightLineNum, leftCount)
					}
				case differ.LineInsert:
					rightCount++
					if l.LeftLineNum != 0 || l.RightLineNum != rightCount {
						t.Errorf("inserted line %v numbering mismatch: got left %d, right %d; want 0, %d", l, l.LeftLineNum, l.RightLineNum, rightCount)
					}
				}
			}
		})
	}
}

func TestDiffFile(t *testing.T) {
	tempDir := t.TempDir()
	leftFile := filepath.Join(tempDir, "left.go")
	rightFile := filepath.Join(tempDir, "right.go")

	leftCode := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	rightCode := "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n\tprintln(\"new line\")\n}\n"

	if err := os.WriteFile(leftFile, []byte(leftCode), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rightFile, []byte(rightCode), 0644); err != nil {
		t.Fatal(err)
	}

	fd, err := differ.DiffFile(differ.ScannedInput{
		RelativePath: "main.go",
		Status:       differ.StatusModified,
		IsBinary:     false,
		LeftPath:     leftFile,
		RightPath:    rightFile,
	})
	if err != nil {
		t.Fatalf("DiffFile error: %v", err)
	}

	if fd.RelativePath != "main.go" {
		t.Errorf("expected relative path main.go, got %s", fd.RelativePath)
	}
	if fd.Additions != 2 || fd.Deletions != 1 {
		t.Errorf("expected 2 additions and 1 deletion, got additions=%d deletions=%d", fd.Additions, fd.Deletions)
	}
}

func TestDiffBinaryFile(t *testing.T) {
	fd, err := differ.DiffFile(differ.ScannedInput{
		RelativePath: "image.png",
		Status:       differ.StatusBinary,
		IsBinary:     true,
		LeftSize:     1024,
		RightSize:    2048,
	})
	if err != nil {
		t.Fatalf("DiffFile error: %v", err)
	}

	if len(fd.Lines) != 0 {
		t.Errorf("expected 0 lines for binary diff, got %d", len(fd.Lines))
	}
	if fd.Additions != 0 || fd.Deletions != 0 {
		t.Errorf("expected 0 additions and 0 deletions for binary diff, got +%d -%d", fd.Additions, fd.Deletions)
	}
}
