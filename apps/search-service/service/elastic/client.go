package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is a minimal Elasticsearch HTTP client (no official SDK — keeps deps light).
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(url string) *Client {
	return &Client{baseURL: url, httpClient: &http.Client{}}
}

// IndexDocument upserts a document into the given index.
func (c *Client) IndexDocument(ctx context.Context, index, id string, doc interface{}) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshaling document: %w", err)
	}
	url := fmt.Sprintf("%s/%s/_doc/%s", c.baseURL, index, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("indexing document: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("elasticsearch returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// SearchHit is a minimal hit returned by Elasticsearch.
type SearchHit struct {
	Index  string          `json:"_index"`
	ID     string          `json:"_id"`
	Score  float64         `json:"_score"`
	Source json.RawMessage `json:"_source"`
}

// Search executes a multi-match query across the given index.
func (c *Client) Search(ctx context.Context, index, query string, page, pageSize int) ([]SearchHit, int, error) {
	from := (page - 1) * pageSize
	payload := map[string]interface{}{
		"from": from,
		"size": pageSize,
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  query,
				"fields": []string{"title^3", "description", "name"},
			},
		},
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/%s/_search", c.baseURL, index)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("searching: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []SearchHit `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("decoding search response: %w", err)
	}
	return result.Hits.Hits, result.Hits.Total.Value, nil
}
