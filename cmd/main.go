package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Println("usage: your_git.sh <command> [<args>]")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		err := gitInit()
		if err != nil {
			fmt.Printf("fatal %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Initialized empty GIT repository")

	case "hash-object":
		if len(os.Args) < 3 {
			fmt.Println("usage: your_git.sh hash-object <file>")
			os.Exit(1)
		}
		file := os.Args[2]
		err := hashObject(file)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "cat-file":
		if len(os.Args) < 3 || os.Args[2] != "-p" {
			fmt.Println("usage: your_git.sh cat-file -p <hash>")
			os.Exit(1)
		}
		objectHash := os.Args[3]
		if err := catFile(objectHash); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	default:
		fmt.Printf("unknown command: %s\n", command)
		os.Exit(1)
	}

}

func gitInit() error {

	gitDir := ".git"

	if err := os.Mkdir(gitDir, os.ModePerm); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("Could not create .git directory: %w", err)
		}
	}

	dirs := []string{"objects", "refs", "refs/heads"}

	for _, d := range dirs {
		fullPath := filepath.Join(gitDir, d)
		if err := os.Mkdir(fullPath, 0755); err != nil {
			return fmt.Errorf("could not create %s: %w", d, err)
		}
	}

	headPath := filepath.Join(gitDir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), os.ModePerm); err != nil {
		return fmt.Errorf("Could not write HEAD file: %w", err)
	}

	return nil
}

func hashObject(fileName string) error {

	content, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("failed to read from file: %w", err)
	}

	header := fmt.Sprintf("blob %d\000", len(content))
	fullContent := append([]byte(header), content...)

	var zlibCompressed bytes.Buffer

	w := zlib.NewWriter(&zlibCompressed)
	_, err = w.Write(fullContent)
	if err != nil {
		return fmt.Errorf("failed to compress using zlib: %w", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close the zlib writer: %w", err)
	}

	hasher := sha1.New()
	_, err = hasher.Write(zlibCompressed.Bytes())
	if err != nil {
		return fmt.Errorf("failed to perform SHA-1 hash: %w", err)
	}
	hashBytes := hasher.Sum(nil)

	hashHex := fmt.Sprintf("%x", hashBytes)

	objectDir := filepath.Join(".git", "objects", hashHex[:2])
	objectPath := filepath.Join(objectDir, hashHex[2:])

	if err := os.MkdirAll(objectDir, 0755); err != nil {
		return fmt.Errorf("failed to create object dir %s: %w", objectDir, err)
	}

	if err := os.WriteFile(objectPath, zlibCompressed.Bytes(), 0755); err != nil {
		return fmt.Errorf("failed to write in .git/objects %w", err)
	}

	fmt.Println(hashHex)

	return nil
}

func catFile(hash string) error {

	objDir := filepath.Join(".git", "objects", hash[:2])
	objFile := filepath.Join(objDir, hash[2:])
	contentBytes, err := os.ReadFile(objFile)
	if err != nil {
		return fmt.Errorf("failed to read contentBytes from %s : %w", objFile, err)
	}

	decompress := bytes.NewReader(contentBytes)
	z, err := zlib.NewReader(decompress)
	if err != nil {
		return fmt.Errorf("zlib new reader: %w", err)
	}
	defer z.Close()

	data, err := io.ReadAll(z)
	if err != nil {
		return fmt.Errorf("failed to read from zlib: %w", err)
	}

	nullIndex := bytes.IndexByte(data, 0)
	if nullIndex == -1 {
		return fmt.Errorf("invalid object format: no null byte found")
	}

	contentStr := data[nullIndex+1:]

	fmt.Println(string(contentStr))

	return nil
}
