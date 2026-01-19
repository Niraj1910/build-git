package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/niraj1910/build-GIT/objects"
	"github.com/niraj1910/build-GIT/tree"
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
		hashHex, zlibCompressedBytes, err := objects.HashBlob(file)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		err = objects.WriteToGitObjects(hashHex, zlibCompressedBytes)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println(hashHex)

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
		if err := tree.LsTree(treeHash); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "write-tree":
		if len(os.Args) < 2 {
			fmt.Println("usage: your_git.sh write-tree")
		}

		result, err := tree.WriteTree(".")
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

		commitHash, err := tree.CommitTree(treeHash, parentTreeHash, message)
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

func catFile(hash string) error {

	contentBytes, err := objects.ReadHashFile(hash)
	if err != nil {
		return err
	}

	data, err := objects.DecompressZlib(contentBytes)
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
