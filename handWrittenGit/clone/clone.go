package clone

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func FetchRefs(url string) error {
	// doing smart http call to get refs
	refUrl := url + "/info/refs?service=git-upload-pack"

	client := &http.Client{}

	req, err := http.NewRequest("GET", refUrl, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w\n", err)
	}

	req.Header.Add("User-Agent", "git/2.39.2")
	req.Header.Add("Accept", "application/x-git-upload-pack-advertisement")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w\n", err)
	}

	defer resp.Body.Close()

	if !strings.Contains(resp.Header.Get("Content-Type"), "git-upload-pack-advertisement") {
		return fmt.Errorf("fatal: dumb protocol not supported\n")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w\n", err)
	}

	fmt.Println("-----------Response body------------")
	fmt.Println(string(body))

	refs, defaultBranch, headCommit, err := ParsePktLine(body)
	if err != nil {
		fmt.Printf("fatal: failed to parse refs: %v\n", err)

	}

	fmt.Println("-----------pkt line response------------")
	fmt.Printf("Default branch: %s\n", defaultBranch)
	fmt.Printf("Commit to fetch: %s\n", headCommit)
	fmt.Printf("All refs: %v\n", refs)

	packData, err := RequestPackFile(url, headCommit)
	if err != nil {
		fmt.Printf("fatal: packfile request failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("-----------packfile response------------")

	fmt.Printf("Packfile received (%d bytes)\n", len(packData))

	return nil
}

func ParsePktLine(body []byte) (map[string]string, string, string, error) {
	refs := make(map[string]string)
	var headCommit string
	var defaultBranch string

	pos := 0
	for pos < len(body) {
		if pos+4 > len(body) {
			break
		}
		lengthHex := string(body[pos : pos+4])
		length, err := strconv.ParseInt(lengthHex, 16, 32)
		if err != nil {
			return nil, "", "", fmt.Errorf("invalid pkt-line length: %s", lengthHex)
		}
		pos += 4

		if length == 0 {

			continue
		}

		if pos+int(length)-4 > len(body) {
			return nil, "", "", fmt.Errorf("pkt-line truncated")
		}
		line := body[pos : pos+int(length)-4]
		pos += int(length) - 4

		lineStr := string(line)

		if strings.HasPrefix(lineStr, "# service=") {
			continue
		}

		parts := strings.SplitN(lineStr, " ", 2)

		if len(parts) < 2 {
			continue
		}

		hash := parts[0]
		rest := parts[1]

		// Handle HEAD line
		if strings.Contains(lineStr, "HEAD") {
			headCommit = hash

			// Find symref anywhere after HEAD
			symrefIndex := strings.Index(lineStr, "symref=HEAD:")
			if symrefIndex != -1 {
				symrefPart := lineStr[symrefIndex:]
				end := strings.Index(symrefPart, " ")
				if end == -1 {
					end = len(symrefPart)
				}
				symrefValue := symrefPart[len("symref=HEAD:"):end]
				defaultBranch = strings.TrimSpace(symrefValue)
			}
		}

		// Branch refs
		if strings.HasPrefix(rest, "refs/heads/") {
			refs[rest] = hash
		}
	}

	if headCommit == "" {
		return nil, "", "", fmt.Errorf("no HEAD commit found in refs")
	}

	// If symref found, use the branch's hash
	if defaultBranch != "" {
		if hash, ok := refs[defaultBranch]; ok {
			headCommit = hash
		}
	}

	return refs, defaultBranch, headCommit, nil
}

func RequestPackFile(baseURL, wantHash string) ([]byte, error) {
	packURL := baseURL + "/git-upload-pack"

	var buf bytes.Buffer

	// Capabilities string - match what your server advertised / common safe set
	caps := "multi_ack thin-pack side-band side-band-64k ofs-delta shallow no-progress include-tag multi_ack_detailed symref object-format=sha1"

	// First (and only) want line with capabilities
	wantLine := fmt.Sprintf("want %s %s\n", wantHash, caps)
	wantPktLen := len(wantLine) + 4 // 4 bytes for the length prefix itself
	fmt.Fprintf(&buf, "%04x%s", wantPktLen, wantLine)

	// No have lines → directly flush + done

	// Flush-pkt (ends the want/have phase)
	buf.WriteString("0000")

	// done pkt-line
	doneLine := "done\n"
	donePktLen := len(doneLine) + 4
	fmt.Fprintf(&buf, "%04x%s", donePktLen, doneLine)

	requestBody := buf.Bytes()

	// Debug print - very important right now
	fmt.Println("--- DEBUG: Pack request body (as string) ---")
	fmt.Println(string(requestBody))
	fmt.Printf("Length: %d bytes\n", len(requestBody))
	fmt.Println("--- End debug ---")

	req, err := http.NewRequest("POST", packURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}

	req.Header.Set("User-Agent", "git/2.43.0")
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	req.Header.Set("Accept", "application/x-git-upload-pack-result")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %s\nBody: %s", resp.Status, string(body))
	}

	fullBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	packStart := bytes.Index(fullBody, []byte("PACK"))
	packData := fullBody[packStart:]

	if len(packData) < 12 {
		return nil, fmt.Errorf("packFile too short")
	}

	if string(packData[0:4]) != "PACK" || binary.BigEndian.Uint32(packData[4:8]) != 2 { // version 2 is most common
		return nil, fmt.Errorf("invalid pack header: % x", packData[:12])
	}

	objCount := binary.BigEndian.Uint32(packData[8:12])
	fmt.Printf("Pack version: %d, contains %d objects\n", 2, objCount)

	return packData, nil
}

func TypeOfObject(objTyp int) string {
	typeName := "unknown"
	switch objTyp {
	case 1:
		typeName = "commit"
	case 2:
		typeName = "tree"
	case 3:
		typeName = "blob"
	case 4:
		typeName = "tag"
	case 6:
		typeName = "ofs-delta"
	case 7:
		typeName = "ref-delta"
	}
	return typeName
}
