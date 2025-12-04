package lcu

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	DefaultLockfilePath = `C:\Riot Games\League of Legends\lockfile`
)

type LockfileInfo struct {
	ProcessName string
	PID         string
	Port        string
	Password    string
	Protocol    string
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	AuthHeader string
}

func GetLockfilePath() string {
	if path := os.Getenv("LOL_LOCKFILE_PATH"); path != "" {
		return path
	}
	return DefaultLockfilePath
}

func ParseLockfile(path string) (*LockfileInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read lockfile: %w", err)
	}

	content := strings.TrimSpace(string(data))
	parts := strings.Split(content, ":")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid lockfile format: expected 5 parts, got %d", len(parts))
	}

	return &LockfileInfo{
		ProcessName: parts[0],
		PID:         parts[1],
		Port:        parts[2],
		Password:    parts[3],
		Protocol:    parts[4],
	}, nil
}

func NewClient(lockfileInfo *LockfileInfo) (*Client, error) {
	baseURL := fmt.Sprintf("%s://127.0.0.1:%s", lockfileInfo.Protocol, lockfileInfo.Port)
	
	// Create Basic Auth header
	auth := fmt.Sprintf("riot:%s", lockfileInfo.Password)
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	// Create HTTP client that trusts self-signed certs
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	return &Client{
		BaseURL:    baseURL,
		HTTPClient: httpClient,
		AuthHeader: authHeader,
	}, nil
}

func (c *Client) Get(endpoint string) ([]byte, error) {
	url := c.BaseURL + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", c.AuthHeader)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, nil
}

