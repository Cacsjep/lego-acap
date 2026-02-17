package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// LogFunc is a function that logs a message (used to pass syslog from app)
type LogFunc func(format string, a ...interface{})

const (
	legoGitHubAPI = "https://api.github.com/repos/go-acme/lego/releases/latest"
	legoBinaryDir = "./localdata"
	legoBinaryPath = "./localdata/lego"
	legoCertsPath  = "./localdata/certs"
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
}

func GetLatestLegoVersion() (string, error) {
	resp, err := http.Get(legoGitHubAPI)
	if err != nil {
		return "", fmt.Errorf("failed to query GitHub API: %w", err)
	}
	defer resp.Body.Close()

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse GitHub response: %w", err)
	}
	return release.TagName, nil
}

func buildDownloadURL(tag string) string {
	return fmt.Sprintf(
		"https://github.com/go-acme/lego/releases/download/%s/lego_%s_linux_%s.tar.gz",
		tag, tag, LegoArch,
	)
}

func IsLegoReady() bool {
	_, err := os.Stat(legoBinaryPath)
	return err == nil
}

func DownloadLego(hub *WSHub) error {
	hub.Broadcast("download_progress", map[string]interface{}{
		"message": "Fetching latest lego version...",
		"percent": 0,
	})

	tag, err := GetLatestLegoVersion()
	if err != nil {
		hub.Broadcast("download_error", map[string]string{"error": err.Error()})
		return err
	}

	url := buildDownloadURL(tag)
	hub.Broadcast("download_progress", map[string]interface{}{
		"message": fmt.Sprintf("Downloading lego %s for %s...", tag, LegoArch),
		"percent": 5,
	})

	resp, err := http.Get(url)
	if err != nil {
		hub.Broadcast("download_error", map[string]string{"error": err.Error()})
		return fmt.Errorf("failed to download lego: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("download failed with status %d", resp.StatusCode)
		hub.Broadcast("download_error", map[string]string{"error": err.Error()})
		return err
	}

	if err := os.MkdirAll(legoBinaryDir, 0755); err != nil {
		hub.Broadcast("download_error", map[string]string{"error": err.Error()})
		return err
	}

	totalSize := resp.ContentLength
	var downloaded int64

	pr := &progressReader{
		reader: resp.Body,
		total:  totalSize,
		onProgress: func(n int64) {
			downloaded += n
			percent := 5
			if totalSize > 0 {
				percent = 5 + int(float64(downloaded)/float64(totalSize)*85)
			}
			hub.Broadcast("download_progress", map[string]interface{}{
				"message":    fmt.Sprintf("Downloading... %d / %d bytes", downloaded, totalSize),
				"percent":    percent,
				"bytes":      downloaded,
				"total":      totalSize,
			})
		},
	}

	hub.Broadcast("download_progress", map[string]interface{}{
		"message": "Extracting lego binary...",
		"percent": 90,
	})

	if err := extractLegoBinary(pr); err != nil {
		hub.Broadcast("download_error", map[string]string{"error": err.Error()})
		return err
	}

	if err := os.Chmod(legoBinaryPath, 0755); err != nil {
		hub.Broadcast("download_error", map[string]string{"error": err.Error()})
		return err
	}

	if err := extractDNSProviders(); err != nil {
		hub.Broadcast("lego_output", map[string]string{
			"line": fmt.Sprintf("Warning: could not extract DNS providers: %s", err),
		})
	}

	hub.Broadcast("download_complete", map[string]string{
		"message": fmt.Sprintf("Lego %s downloaded successfully", tag),
		"version": tag,
	})
	return nil
}

const providersPath = "./localdata/providers.json"

func extractDNSProviders() error {
	cmd := exec.Command(legoBinaryPath, "dnshelp")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run lego dnshelp: %w (output: %s)", err, string(output))
	}

	// Find the line after "Supported DNS providers:" which is a comma-separated list
	text := string(output)
	marker := "Supported DNS providers:"
	idx := strings.Index(text, marker)
	if idx < 0 {
		return fmt.Errorf("could not find provider list in dnshelp output")
	}
	rest := text[idx+len(marker):]

	var providers []string
	for _, name := range strings.Split(rest, ",") {
		name = strings.TrimSpace(name)
		// Stop at empty lines or "More information:" footer
		if name == "" || strings.HasPrefix(name, "More") || strings.HasPrefix(name, "http") {
			continue
		}
		// Clean up any newlines within the comma list
		name = strings.TrimSpace(strings.ReplaceAll(name, "\n", ""))
		if name != "" {
			providers = append(providers, name)
		}
	}

	if len(providers) == 0 {
		return fmt.Errorf("no providers found in dnshelp output")
	}

	data, err := json.Marshal(providers)
	if err != nil {
		return err
	}
	return os.WriteFile(providersPath, data, 0644)
}

func GetDNSProviders() ([]string, error) {
	data, err := os.ReadFile(providersPath)
	if err != nil {
		return nil, err
	}
	var providers []string
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, err
	}
	return providers, nil
}

func extractLegoBinary(reader io.Reader) error {
	gzr, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}
		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == "lego" {
			outFile, err := os.Create(legoBinaryPath)
			if err != nil {
				return fmt.Errorf("failed to create lego binary: %w", err)
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tr); err != nil {
				return fmt.Errorf("failed to write lego binary: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("lego binary not found in archive")
}

type progressReader struct {
	reader     io.Reader
	total      int64
	onProgress func(n int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 && pr.onProgress != nil {
		pr.onProgress(int64(n))
	}
	return n, err
}

var (
	legoCmd    *exec.Cmd
	legoCancel context.CancelFunc
	legoCmdMu  sync.Mutex
)

func StopLego(hub *WSHub) error {
	legoCmdMu.Lock()
	defer legoCmdMu.Unlock()
	if legoCmd == nil || legoCmd.Process == nil {
		return fmt.Errorf("no lego process running")
	}
	legoCancel()
	hub.Broadcast("lego_output", map[string]string{"line": "--- Process stopped by user ---"})
	return nil
}

func IsLegoRunning() bool {
	legoCmdMu.Lock()
	defer legoCmdMu.Unlock()
	return legoCmd != nil && legoCmd.Process != nil
}

func RunLego(config *Config, hub *WSHub, command string, logf LogFunc) (string, error) {
	if !IsLegoReady() {
		return "", fmt.Errorf("lego binary not found, please download first")
	}

	args := []string{
		"--email", config.Email,
		"--dns", config.DNSProvider,
		"--accept-tos",
		"--path", legoCertsPath,
	}

	domains := strings.Split(config.Domains, ",")
	for _, d := range domains {
		d = strings.TrimSpace(d)
		if d != "" {
			args = append(args, "--domains", d)
		}
	}

	if config.DNSResolvers != "" {
		args = append(args, "--dns.resolvers", config.DNSResolvers)
	}
	if config.CAServer != "" {
		args = append(args, "--server", config.CAServer)
	}
	if config.KeyType != "" {
		args = append(args, "--key-type", config.KeyType)
	}
	if config.EABEnabled && config.EABKID != "" && config.EABHMAC != "" {
		args = append(args, "--eab", "--kid", config.EABKID, "--hmac", config.EABHMAC)
	}

	switch command {
	case "obtain":
		args = append(args, "run")
	case "renew":
		args = append(args, "renew", "--days", "30")
	default:
		return "", fmt.Errorf("unknown command: %s", command)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, legoBinaryPath, args...)

	legoCmdMu.Lock()
	legoCmd = cmd
	legoCancel = cancel
	legoCmdMu.Unlock()

	defer func() {
		legoCmdMu.Lock()
		legoCmd = nil
		legoCancel = nil
		legoCmdMu.Unlock()
		cancel()
	}()

	envVars := make(map[string]string)
	if err := json.Unmarshal([]byte(config.EnvVars), &envVars); err != nil {
		return "", fmt.Errorf("failed to parse env vars: %w", err)
	}

	cmd.Env = os.Environ()
	for k, v := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	var outputBuf strings.Builder

	cmdLine := fmt.Sprintf("Running: lego %s", strings.Join(args, " "))
	hub.Broadcast("lego_output", map[string]string{"line": cmdLine})
	logf("[lego] %s", cmdLine)
	outputBuf.WriteString(cmdLine + "\n")

	if err := cmd.Start(); err != nil {
		hub.Broadcast("lego_error", map[string]string{"error": err.Error()})
		return outputBuf.String(), fmt.Errorf("failed to start lego: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		hub.Broadcast("lego_output", map[string]string{"line": line})
		logf("[lego] %s", line)
		outputBuf.WriteString(line + "\n")
	}

	output := outputBuf.String()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			logf("[lego] Process stopped by user")
			return output, nil
		}
		hub.Broadcast("lego_error", map[string]string{"error": err.Error()})
		return output, fmt.Errorf("lego exited with error: %w", err)
	}

	msg := fmt.Sprintf("Certificate %s completed successfully", command)
	hub.Broadcast("lego_complete", map[string]string{"message": msg})
	logf("[lego] %s", msg)
	return output, nil
}
