package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mhai-org/term-ai/internal/ai"
)

func init() {
	register("env_list", envListDef(), runEnvList)
	register("git_status", gitStatusDef(), runGitStatus)
	register("http_get", httpGetDef(), runHTTPGet)
	register("json_query", jsonQueryDef(), runJSONQuery)
	register("hash", hashDef(), runHash)
	register("base64", base64Def(), runBase64)
	register("make_dir", makeDirDef(), runMakeDir)
}

// ---- env_list -----------------------------------------------------------

// secretKeyPatterns lists substrings (uppercased) that indicate a sensitive env var value.
var secretKeyPatterns = []string{
	"KEY", "TOKEN", "SECRET", "PASSWORD", "PASS", "CREDENTIAL", "AUTH", "CERT", "PRIVATE", "PWD",
}

func isSecretKey(key string) bool {
	up := strings.ToUpper(key)
	for _, p := range secretKeyPatterns {
		if strings.Contains(up, p) {
			return true
		}
	}
	return false
}

func envListDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "If provided, return only the value of this specific variable. Values of sensitive keys are redacted."}
		}
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "env_list",
			Description: "List environment variables. Values of keys that look sensitive (KEY, TOKEN, SECRET, PASSWORD, etc.) are shown as [redacted]. Optionally filter to a single variable by name.",
			Parameters:  params,
		},
	}
}

func runEnvList(argsJSON string) (string, error) {
	var args struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &args)

	if args.Name != "" {
		val, ok := os.LookupEnv(args.Name)
		if !ok {
			return fmt.Sprintf("%s is not set", args.Name), nil
		}
		if isSecretKey(args.Name) {
			return fmt.Sprintf("%s=[redacted]", args.Name), nil
		}
		return fmt.Sprintf("%s=%s", args.Name, val), nil
	}

	var sb strings.Builder
	for _, entry := range os.Environ() {
		idx := strings.IndexByte(entry, '=')
		if idx < 0 {
			sb.WriteString(entry + "\n")
			continue
		}
		key := entry[:idx]
		val := entry[idx+1:]
		if isSecretKey(key) {
			val = "[redacted]"
		}
		fmt.Fprintf(&sb, "%s=%s\n", key, val)
	}
	return sb.String(), nil
}

// ---- git_status ---------------------------------------------------------

func gitStatusDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory to run git in (default: current directory)."}
		}
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "git_status",
			Description: "Return git status (short format) and the last 5 commits for a repository.",
			Parameters:  params,
		},
	}
}

func runGitStatus(argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	args.Path = "."
	_ = json.Unmarshal([]byte(argsJSON), &args)
	if args.Path == "" {
		args.Path = "."
	}

	run := func(gitArgs ...string) string {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var buf bytes.Buffer
		a := append([]string{"-C", args.Path}, gitArgs...)
		cmd := exec.CommandContext(ctx, "git", a...)
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		cmd.Run() //nolint:errcheck
		return buf.String()
	}

	status := run("status", "--short")
	log := run("log", "--oneline", "-5")
	branch := run("branch", "--show-current")

	return fmt.Sprintf("Branch: %s\n=== Status ===\n%s\n=== Recent Commits ===\n%s",
		strings.TrimSpace(branch), status, log), nil
}

// ---- http_get -----------------------------------------------------------

// privateRanges lists CIDR blocks that must not be targeted (SSRF protection).
var privateRanges []net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // RFC-1918
		"172.16.0.0/12",  // RFC-1918
		"192.168.0.0/16", // RFC-1918
		"169.254.0.0/16", // link-local
		"100.64.0.0/10",  // shared address space (RFC-6598)
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	}
	for _, c := range cidrs {
		_, network, err := net.ParseCIDR(c)
		if err == nil {
			privateRanges = append(privateRanges, *network)
		}
	}
}

func isPrivateIP(ip net.IP) bool {
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

// checkSSRF resolves the host of rawURL and rejects private/loopback addresses.
func checkSSRF(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}

	// Direct IP check.
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("SSRF protection: %s is a private/loopback address", host)
		}
		return nil
	}

	// Resolve hostname; if resolution fails, let the HTTP client handle it.
	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil
	}
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil && isPrivateIP(ip) {
			return fmt.Errorf("SSRF protection: %s resolves to a private/loopback address (%s)", host, addr)
		}
	}
	return nil
}

func httpGetDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url":     {"type": "string",  "description": "The URL to GET. Private/loopback addresses are blocked."},
			"headers": {"type": "object",  "description": "Optional request headers as key-value pairs.",
				"additionalProperties": {"type": "string"}}
		},
		"required": ["url"]
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "http_get",
			Description: "Perform an HTTP GET request and return the status code and response body (capped at 20 KB). Requests to private/loopback addresses are blocked.",
			Parameters:  params,
		},
	}
}

func runHTTPGet(argsJSON string) (string, error) {
	var args struct {
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("http_get: invalid args: %w", err)
	}
	if args.URL == "" {
		return "", fmt.Errorf("http_get: url must not be empty")
	}
	if err := checkSSRF(args.URL); err != nil {
		return "", fmt.Errorf("http_get: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, args.URL, nil)
	if err != nil {
		return "", fmt.Errorf("http_get: %w", err)
	}
	for k, v := range args.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http_get: request failed: %w", err)
	}
	defer resp.Body.Close()

	const maxBody = 20 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return "", fmt.Errorf("http_get: reading response: %w", err)
	}

	truncated := ""
	if len(body) > maxBody {
		body = body[:maxBody]
		truncated = "\n... (truncated)"
	}
	return fmt.Sprintf("HTTP %d\n\n%s%s", resp.StatusCode, string(body), truncated), nil
}

// ---- json_query ---------------------------------------------------------

func jsonQueryDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"json": {"type": "string", "description": "A valid JSON string to query."},
			"path": {"type": "string", "description": "Dot-separated key path to extract (e.g. 'user.address.city'). Leave empty to return the entire document."}
		},
		"required": ["json"]
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "json_query",
			Description: "Parse a JSON string and extract a value at a dot-separated key path. Returns the result as pretty-printed JSON.",
			Parameters:  params,
		},
	}
}

func runJSONQuery(argsJSON string) (string, error) {
	var args struct {
		JSON string `json:"json"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("json_query: invalid args: %w", err)
	}

	var data interface{}
	if err := json.Unmarshal([]byte(args.JSON), &data); err != nil {
		return "", fmt.Errorf("json_query: invalid JSON input: %w", err)
	}

	current := data
	if args.Path != "" {
		for _, key := range strings.Split(args.Path, ".") {
			if key == "" {
				continue
			}
			m, ok := current.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("json_query: cannot index into %T at key %q", current, key)
			}
			val, exists := m[key]
			if !exists {
				return "", fmt.Errorf("json_query: key %q not found", key)
			}
			current = val
		}
	}

	out, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json_query: serializing result: %w", err)
	}
	return string(out), nil
}

// ---- hash ---------------------------------------------------------------

func hashDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"type":    {"type": "string", "enum": ["file", "text"], "description": "Whether to hash a file or an inline text string."},
			"value":   {"type": "string", "description": "File path (for type=file) or the text string to hash (for type=text)."}
		},
		"required": ["type", "value"]
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "hash",
			Description: "Compute the SHA-256 hash of a file or a text string.",
			Parameters:  params,
		},
	}
}

func runHash(argsJSON string) (string, error) {
	var args struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("hash: invalid args: %w", err)
	}

	var data []byte
	switch args.Type {
	case "text":
		data = []byte(args.Value)
	case "file":
		if isHiddenPath(args.Value) {
			return "", fmt.Errorf("hash: access to hidden files is not permitted")
		}
		var err error
		data, err = os.ReadFile(args.Value)
		if err != nil {
			return "", fmt.Errorf("hash: %w", err)
		}
	default:
		return "", fmt.Errorf("hash: type must be 'file' or 'text'")
	}

	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256: %x", sum), nil
}

// ---- base64 -------------------------------------------------------------

func base64Def() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"operation": {"type": "string", "enum": ["encode", "decode"], "description": "Whether to encode or decode."},
			"value":     {"type": "string", "description": "The string to encode or the base64 string to decode."}
		},
		"required": ["operation", "value"]
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "base64",
			Description: "Encode a string to base64 or decode a base64 string.",
			Parameters:  params,
		},
	}
}

func runBase64(argsJSON string) (string, error) {
	var args struct {
		Operation string `json:"operation"`
		Value     string `json:"value"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("base64: invalid args: %w", err)
	}
	switch args.Operation {
	case "encode":
		return base64.StdEncoding.EncodeToString([]byte(args.Value)), nil
	case "decode":
		decoded, err := base64.StdEncoding.DecodeString(args.Value)
		if err != nil {
			return "", fmt.Errorf("base64: decode error: %w", err)
		}
		return string(decoded), nil
	default:
		return "", fmt.Errorf("base64: operation must be 'encode' or 'decode'")
	}
}

// ---- make_dir -----------------------------------------------------------

// blockedRoots lists path prefixes where directory creation is not allowed.
var blockedRoots = []string{
	"/etc", "/usr", "/bin", "/sbin", "/boot", "/sys", "/proc", "/dev", "/lib", "/lib64",
}

func isBlockedPath(path string) bool {
	clean := filepath.Clean(path)
	for _, b := range blockedRoots {
		if clean == b || strings.HasPrefix(clean, b+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func makeDirDef() ai.ToolDefinition {
	params := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path to create (including any parents, like mkdir -p)."}
		},
		"required": ["path"]
	}`)
	return ai.ToolDefinition{
		Type: "function",
		Function: ai.ToolFunction{
			Name:        "make_dir",
			Description: "Create a directory (and any missing parents). Blocked for sensitive system paths (/etc, /usr, /bin, /sys, /proc, etc.).",
			Parameters:  params,
		},
	}
}

func runMakeDir(argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("make_dir: invalid args: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("make_dir: path must not be empty")
	}
	if isBlockedPath(args.Path) {
		return "", fmt.Errorf("make_dir: creation in %s is not permitted", args.Path)
	}
	if err := os.MkdirAll(args.Path, 0755); err != nil {
		return "", fmt.Errorf("make_dir: %w", err)
	}
	return fmt.Sprintf("created directory: %s", filepath.Clean(args.Path)), nil
}
