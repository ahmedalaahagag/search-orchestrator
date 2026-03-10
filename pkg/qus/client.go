package qus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ahmedalaahagag/search-orchestrator/pkg/model"
)

type QUSClient interface {
	Analyze(ctx context.Context, req model.QUSAnalyzeRequest) (*model.QUSAnalyzeResponse, error)
}

type ClientConfig struct {
	BaseURL string
	Timeout time.Duration
}

type Client struct {
	cfg  ClientConfig
	http *http.Client
}

func NewClient(cfg ClientConfig) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 3 * time.Second
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (c *Client) Analyze(ctx context.Context, req model.QUSAnalyzeRequest) (*model.QUSAnalyzeResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshalling QUS request: %w", err)
	}

	url := c.cfg.BaseURL + "/v1/analyze"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating QUS request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing QUS request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("QUS returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result model.QUSAnalyzeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding QUS response: %w", err)
	}

	return &result, nil
}
