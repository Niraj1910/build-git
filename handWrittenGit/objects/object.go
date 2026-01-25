package objects

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Hash(raw []byte) ([]byte, error) {
	hasher := sha1.New()
	_, err := hasher.Write(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to perform SHA-1 hash: %w", err)
	}
	hashBytes := hasher.Sum(nil)
	return hashBytes, nil
}

func ReadHashFile(hash string) ([]byte, error) {
	objDir := filepath.Join(".git", "objects", hash[:2])
	objFile := filepath.Join(objDir, hash[2:])
	hashedBytes, err := os.ReadFile(objFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read hashedBytes from %s : %w", objFile, err)
	}
	return hashedBytes, nil
}

func Compress(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	z := zlib.NewWriter(&buffer)
	_, err := z.Write(data)
	if err != nil {
		return nil, fmt.Errorf("failed to compress using zlib: %w", err)
	}
	if err := z.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zlib writer: %w", err)
	}
	return buffer.Bytes(), nil
}

func DecompressZlib(data []byte) ([]byte, error) {
	compressed := bytes.NewReader(data)
	z, err := zlib.NewReader(compressed)
	if err != nil {
		return nil, fmt.Errorf("zlib new reader: %w", err)
	}
	defer z.Close()

	data, err = io.ReadAll(z)
	if err != nil {
		return nil, fmt.Errorf("failed to read from zlib: %w", err)
	}
	return data, nil
}

func WriteToGitObjects(hashHex string, compressedBytes []byte) error {

	objectDir := filepath.Join(".git", "objects", hashHex[:2])
	objectPath := filepath.Join(objectDir, hashHex[2:])

	if err := os.MkdirAll(objectDir, 0755); err != nil {
		return fmt.Errorf("failed to create object dir %s: %w", objectDir, err)
	}

	if err := os.WriteFile(objectPath, compressedBytes, 0755); err != nil {
		return fmt.Errorf("failed to write in .git/objects %w", err)
	}

	return nil
}
