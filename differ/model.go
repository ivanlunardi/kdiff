package differ

import "time"

type FileStatus string

const (
	StatusAdded     FileStatus = "ADDED"
	StatusDeleted   FileStatus = "DELETED"
	StatusModified  FileStatus = "MODIFIED"
	StatusIdentical FileStatus = "IDENTICAL"
	StatusBinary    FileStatus = "BINARY"
)

type LineChangeType string

const (
	LineEqual  LineChangeType = "EQUAL"
	LineInsert LineChangeType = "INSERT"
	LineDelete LineChangeType = "DELETE"
)

type DiffLine struct {
	LeftLineNum  int            `json:"leftLineNum"`
	RightLineNum int            `json:"rightLineNum"`
	Type         LineChangeType `json:"type"`
	Content      string         `json:"content"`
}

type FileDiff struct {
	RelativePath string     `json:"relativePath"`
	Status       FileStatus `json:"status"`
	IsBinary     bool       `json:"isBinary"`
	LeftSize     int64      `json:"leftSize"`
	RightSize    int64      `json:"rightSize"`
	Additions    int        `json:"additions"`
	Deletions    int        `json:"deletions"`
	Lines        []DiffLine `json:"lines"`
}

type ComparisonReport struct {
	ID             string     `json:"id"`
	Timestamp      time.Time  `json:"timestamp"`
	Title          string     `json:"title"`
	LeftPath       string     `json:"leftPath"`
	RightPath      string     `json:"rightPath"`
	TotalFiles     int        `json:"totalFiles"`
	AddedCount     int        `json:"addedCount"`
	DeletedCount   int        `json:"deletedCount"`
	ModifiedCount  int        `json:"modifiedCount"`
	IdenticalCount int        `json:"identicalCount"`
	Files          []FileDiff `json:"files"`
}
