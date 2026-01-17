package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

	case "ls-tree":
		if len(os.Args) < 3 || os.Args[2] != "-p" {
			fmt.Println("usage: your_git.sh ls-tree -p <tree-hash>")
		}
		treeHash := os.Args[3]
		if err := lsTree(treeHash); err != nil {
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

func readHashFile(hash string) ([]byte, error) {
	objDir := filepath.Join(".git", "objects", hash[:2])
	objFile := filepath.Join(objDir, hash[2:])
	hashedBytes, err := os.ReadFile(objFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read hashedBytes from %s : %w", objFile, err)
	}
	return hashedBytes, nil
}

func decompressZlib(data []byte) ([]byte, error) {
	decompress := bytes.NewReader(data)
	z, err := zlib.NewReader(decompress)
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

func catFile(hash string) error {

	contentBytes, err := readHashFile(hash)
	if err != nil {
		return err
	}

	data, err := decompressZlib(contentBytes)
	if err != nil {
		return err
	}

	nullIndex := bytes.IndexByte(data, 0)
	if nullIndex == -1 {
		return fmt.Errorf("invalid object format: no null byte found")
	}

	contentStr := data[nullIndex+1:]

	fmt.Println(string(contentStr))

	return nil
}

func lsTree(treeHash string) error {

	// mode blob hash name
	hashedBytes, err := readHashFile(treeHash)
	if err != nil {
		return err
	}

	decompressedContent, err := decompressZlib(hashedBytes)

	/*
		example of binary content of tree
		100644 test.txt\0[20-byte binary hash for blob]
		040000 docs\0[20-byte binary hash for sub-tree]
	*/

	pos := 0
	for pos < len(decompressedContent) {
		// Find space -> mode ends
		spaceIdx := bytes.IndexByte(decompressedContent[pos:], ' ')
		modeStr := string(decompressedContent[pos : pos+spaceIdx])
		pos = spaceIdx + 1 // skip space

		nullIdx := bytes.IndexByte(decompressedContent[pos:], 0)
		if nullIdx == -1 {
			return fmt.Errorf("invalid tree format: no null byte after name")
		}
		nameStr := string(decompressedContent[pos : pos+nullIdx])

		pos = nullIdx + 1 // slip \0

		// check for 20 bytes = hash
		if pos+20 > len(decompressedContent) {
			break
		}

		hashBin := decompressedContent[pos : pos+20]
		hashHex := fmt.Sprintf("%x", hashBin)

		pos += 20

		// type for blob
		isBlob := "blob"
		if strings.HasPrefix(modeStr, "04") {
			isBlob = "tree"
		}
		fmt.Printf("%s %s %s\t %s", modeStr, isBlob, nameStr, hashHex)
	}

	return nil
}
