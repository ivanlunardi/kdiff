package scanner

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

const maxCheckBytes = 8192

// IsBinaryBuffer inspects a byte slice to determine if it represents binary data.
func IsBinaryBuffer(data []byte) bool {
	limit := len(data)
	if limit > maxCheckBytes {
		limit = maxCheckBytes
	}
	return bytes.IndexByte(data[:limit], 0) != -1
}

// IsBinaryFile inspects the start of a file on disk to determine if it is binary.
func IsBinaryFile(filePath string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("opening file for binary check %s: %w", filePath, err)
	}
	defer f.Close()

	buf := make([]byte, maxCheckBytes)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("reading file for binary check %s: %w", filePath, err)
	}

	return bytes.IndexByte(buf[:n], 0) != -1, nil
}
