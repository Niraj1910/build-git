package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
	case "write-tree":
		if len(os.Args) < 2 {
			fmt.Println("usage: your_git.sh write-tree")
		}

		result, err := writeTree(".")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(result)

	case "commit-tree": // git commit-tree <tree-hash> [-p <parent-tree-hash>] [-m <message>]
		if len(os.Args) < 4 {
			fmt.Println("usage: /your_git.sh commit-tree <tree-hash> [-p <parent>] [-m <message>]")
			os.Exit(1)
		}
		treeHash := os.Args[2]

		var parentTreeHash string
		var message string

		i := 3

		for i < len(os.Args) {
			switch os.Args[i] {
			case "-p":
				i++
				if i < len(os.Args) {
					parentTreeHash = os.Args[i]
				}
			case "-m":
				i++
				if i < len(os.Args) {
					message = os.Args[i]
				}
			}
			i++
		}
		if message == "" {
			fmt.Println("fatal: no message provided (-m)")
			os.Exit(1)
		}

		commitHash, err := commitTree(treeHash, parentTreeHash, message)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(commitHash)

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

	hashHex, zlibCompressedBytes, err := hashBlob(fileName)
	if err != nil {
		return nil
	}

	err = writeToGitObjects(hashHex, zlibCompressedBytes)
	if err != nil {
		return err
	}

	fmt.Println(hashHex)

	return nil
}

func writeToGitObjects(hashHex string, compressedBytes []byte) error {

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
	fmt.Println("Full decompressed length:", len(data))
	fmt.Println("Header:", string(data[:nullIndex]))
	fmt.Println("Content length after header:", len(data[nullIndex+1:]))
	fmt.Println("Content bytes:", data[nullIndex+1:])
	if nullIndex == -1 {
		return fmt.Errorf("invalid object format: no null byte found")
	}

	contentStr := data[nullIndex+1:]

	fmt.Println(string(contentStr))

	return nil
}

func lsTree(treeHash string) error {
	hashedBytes, err := readHashFile(treeHash)
	if err != nil {
		return err
	}

	decompressedContent, err := decompressZlib(hashedBytes)
	if err != nil {
		return err
	}

	nullIndex := bytes.IndexByte(decompressedContent, 0)
	if nullIndex == -1 {
		return fmt.Errorf("invalid tree format: no null byte in header")
	}

	pos := nullIndex + 1
	for pos < len(decompressedContent) {
		spaceIdx := bytes.IndexByte(decompressedContent[pos:], ' ')
		if spaceIdx == -1 {
			return fmt.Errorf("invalid tree format: no space after mode")
		}
		modeStr := string(decompressedContent[pos : pos+spaceIdx])

		pos += spaceIdx + 1

		nullIdx := bytes.IndexByte(decompressedContent[pos:], 0)
		if nullIdx == -1 {
			return fmt.Errorf("invalid tree format: no null byte after name")
		}
		nameStr := string(decompressedContent[pos : pos+nullIdx])

		pos += nullIdx + 1

		if pos+20 > len(decompressedContent) {
			break
		}

		hashBin := decompressedContent[pos : pos+20]
		hashHex := fmt.Sprintf("%x", hashBin)

		pos += 20

		objType := "blob"
		if strings.HasPrefix(modeStr, "04") {
			objType = "tree"
		}

		// Fixed order + newline
		fmt.Printf("%s %s %s\t%s\n", modeStr, objType, hashHex, nameStr)
	}

	return nil
}

func writeTree(dir string) (string, error) {

	// get all the folders/files
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	type treeEntry struct {
		mode string
		name string
		hash string
	}

	var treeEntries []treeEntry

	/*
	   tree <size>\0
	   <mode> <name>\0<20_byte_sha>
	   <mode> <name>\0<20_byte_sha>
	*/

	for _, entry := range entries {

		name := entry.Name()
		if name == ".git" {
			continue
		}

		fullPath := filepath.Join(dir, name)

		if entry.IsDir() {
			subTreeHash, err := writeTree(fullPath)
			if err != nil {
				return "", err
			}
			treeEntries = append(treeEntries, treeEntry{
				mode: "040000",
				name: name,
				hash: subTreeHash,
			})

		} else if entry.Type().IsRegular() {
			blobHash, _, err := hashBlob(fullPath)
			if err != nil {
				return "", err
			}
			treeEntries = append(treeEntries, treeEntry{
				mode: "100644",
				name: name,
				hash: blobHash,
			})
		}
	}
	// sort entries alphabetically
	sort.Slice(treeEntries, func(i, j int) bool {
		return treeEntries[i].name < treeEntries[j].name
	})

	// build tree content
	var treeContent bytes.Buffer
	for _, e := range treeEntries {
		// Mode + space + name + \0 + binary hash (convert hex back to 20 bytes)
		modeBytes := []byte(e.mode + " ")
		nameBytes := []byte(e.name)

		hashBin, _ := hex.DecodeString(e.hash)

		treeContent.Write(modeBytes)
		treeContent.Write(nameBytes)
		treeContent.WriteByte(0) // \0
		treeContent.Write(hashBin)
	}

	// tree header
	treeHeader := fmt.Sprintf("tree %d\000", treeContent.Len())
	fullContent := append([]byte(treeHeader), treeContent.Bytes()...)

	// compress
	compressedTree, err := compress(fullContent)
	if err != nil {
		return "", err
	}
	// hash
	hashedBytes, err := hash(compressedTree)
	if err != nil {
		return "", err
	}
	hashHex := fmt.Sprintf("%x", hashedBytes)

	// write in .git/objects
	err = writeToGitObjects(hashHex, compressedTree)
	if err != nil {
		return "", err
	}
	return hashHex, nil
}

func compress(data []byte) ([]byte, error) {
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

func hash(raw []byte) ([]byte, error) {
	hasher := sha1.New()
	_, err := hasher.Write(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to perform SHA-1 hash: %w", err)
	}
	hashBytes := hasher.Sum(nil)
	return hashBytes, nil
}

func hashBlob(filePath string) (string, []byte, error) {

	// read file path
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// build header "blob <size>\0" + content
	header := fmt.Sprintf("blob %d\000", len(raw))

	// compress using zlib
	compressedBytes, err := compress([]byte(header))
	if err != nil {
		return "", nil, err
	}

	// compute SHA-1 of compressed
	hashBytes, err := hash(compressedBytes)
	if err != nil {
		return "", nil, err
	}

	// return hex string
	hashHex := fmt.Sprintf("%x", hashBytes)

	return hashHex, compressedBytes, nil
}

func commitTree(treeHash, parentHash, message string) (string, error) {

	/*
		commit 177\0tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904
		parent 3b18e512dba79e4c8300dd08aeb37f8e728b8dad
		author John Doe <john@example.com> 1234567890 +0000
		committer John Doe <john@example.com> 1234567890 +0000

		Initial commit
	*/

	var content bytes.Buffer

	content.WriteString(fmt.Sprintf("tree %s\n", treeHash))

	if parentHash != "" {
		content.WriteString(fmt.Sprintf("parent %s\n", parentHash))
	}

	now := time.Now()
	timeStamp := now.Unix()
	offSet := now.Format("-0700")
	author := fmt.Sprintf("Niraj Shaw <niraj@example.com> %d %s", timeStamp, offSet)
	content.WriteString(fmt.Sprintf("author %s\n", author))
	content.WriteString(fmt.Sprintf("committer %s\n", author))

	content.WriteString("\n")

	content.WriteString(message + "\n")

	header := fmt.Sprintf("commit %d\000", content.Len())
	fullContent := append([]byte(header), []byte(content.String())...)

	// compress
	compressedData, err := compress(fullContent)
	if err != nil {
		return "", err
	}

	// hash
	hashedByte, err := hash(compressedData)
	if err != nil {
		return "", err
	}
	hashHex := fmt.Sprintf("%x", hashedByte)

	// write to .git/objects
	err = writeToGitObjects(hashHex, compressedData)
	if err != nil {
		return "", err
	}

	return hashHex, nil
}
