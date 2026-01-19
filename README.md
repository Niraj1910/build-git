# Hand-Written Git Implementation in Go

A minimal Git version control system built from scratch in Go, using only the standard library — no external dependencies.

This project implements core Git commands (`init`, `hash-object`, `cat-file`, `ls-tree`, `write-tree`, `commit-tree`) following the real Git internals: loose objects, zlib compression, SHA-1 hashing, tree parsing, and commit object format.

Inspired by the CodeCrafters Git challenge — built to deeply understand how Git works under the hood.

## Features Implemented

- `git init` — initializes a new empty Git repository
- `git hash-object <file>` — creates a blob object from file content
- `git cat-file -p <hash>` — pretty-prints blob content (skips header)
- `git ls-tree -p <tree-hash>` — lists contents of a tree object (mode, type, hash, name)
- `git write-tree` — recursively builds a tree object from the current directory
- `git commit-tree <tree> [-p <parent>] [-m <message>]` — creates a commit object with tree, optional parent, author, committer, and message

All commands handle real Git loose object format, binary parsing, compression, and hashing.
