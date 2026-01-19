package objects

import (
	"fmt"
	"os"
)

func HashBlob(filePath string) (string, []byte, error) {

	// read file path
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// build header "blob <size>\0" + content
	header := fmt.Sprintf("blob %d\000", len(raw))
	fullContent := append([]byte(header), raw...)

	// compress using zlib
	compressedBytes, err := Compress(fullContent)
	if err != nil {
		return "", nil, err
	}

	// compute SHA-1 of compressed
	hashBytes, err := Hash(compressedBytes)
	if err != nil {
		return "", nil, err
	}

	// return hex string
	hashHex := fmt.Sprintf("%x", hashBytes)

	return hashHex, compressedBytes, nil
}
