package tree

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/niraj1910/build-GIT/objects"
)

func LsTree(treeHash string) error {
	hashedBytes, err := objects.ReadHashFile(treeHash)
	if err != nil {
		return err
	}

	decompressedContent, err := objects.DecompressZlib(hashedBytes)
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

func WriteTree(dir string) (string, error) {

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
			subTreeHash, err := WriteTree(fullPath)
			if err != nil {
				return "", err
			}
			treeEntries = append(treeEntries, treeEntry{
				mode: "040000",
				name: name,
				hash: subTreeHash,
			})

		} else if entry.Type().IsRegular() {
			blobHash, _, err := objects.HashBlob(fullPath)
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
	compressedTree, err := objects.Compress(fullContent)
	if err != nil {
		return "", err
	}
	// hash
	hashedBytes, err := objects.Hash(compressedTree)
	if err != nil {
		return "", err
	}
	hashHex := fmt.Sprintf("%x", hashedBytes)

	// write in .git/objects
	err = objects.WriteToGitObjects(hashHex, compressedTree)
	if err != nil {
		return "", err
	}
	return hashHex, nil
}

func CommitTree(treeHash, parentHash, message string) (string, error) {

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
	compressedData, err := objects.Compress(fullContent)
	if err != nil {
		return "", err
	}

	// hash
	hashedByte, err := objects.Hash(compressedData)
	if err != nil {
		return "", err
	}
	hashHex := fmt.Sprintf("%x", hashedByte)

	// write to .git/objects
	err = objects.WriteToGitObjects(hashHex, compressedData)
	if err != nil {
		return "", err
	}

	return hashHex, nil
}
