package huly

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang/snappy"
)

// RestClient calls the Huly transactor REST API under {endpoint}/api/v1.
type RestClient struct {
	endpoint  string
	workspace string
	token     string
	client    *http.Client
}

func NewRestClient(endpoint, workspace, token string) *RestClient {
	return &RestClient{
		endpoint:  strings.TrimRight(rewriteScheme(endpoint), "/"),
		workspace: workspace,
		token:     token,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *RestClient) do(ctx context.Context, method, path string, q url.Values, body []byte) ([]byte, error) {
	u := c.endpoint + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	for attempt := 0; ; attempt++ {
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, u, rdr)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept-Encoding", "snappy, gzip")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			d := retryAfter(resp.Header)
			resp.Body.Close()
			select {
			case <-time.After(d):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		data, derr := decodeBody(resp)
		resp.Body.Close()
		if derr != nil {
			return nil, derr
		}
		switch resp.StatusCode {
		case http.StatusOK, http.StatusCreated, http.StatusNoContent:
			return data, nil
		case http.StatusUnauthorized:
			return nil, ErrUnauthorized
		case http.StatusNotFound:
			return nil, ErrNotFound
		default:
			return nil, &APIError{Status: resp.StatusCode, Body: string(data)}
		}
	}
}

func retryAfter(h http.Header) time.Duration {
	if ms := h.Get("Retry-After-ms"); ms != "" {
		if n, err := strconv.Atoi(ms); err == nil {
			return time.Duration(n) * time.Millisecond
		}
	}
	if s := h.Get("Retry-After"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return time.Second
}

func decodeBody(resp *http.Response) ([]byte, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	switch resp.Header.Get("Content-Encoding") {
	case "snappy":
		return snappy.Decode(nil, raw)
	case "gzip":
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		return io.ReadAll(zr)
	default:
		return raw, nil
	}
}

// FindAll queries documents of a class; decodes the value array into out.
func (c *RestClient) FindAll(ctx context.Context, class string, query, options map[string]any, out any) (int, error) {
	q := url.Values{}
	q.Set("class", class)
	if query != nil {
		b, _ := json.Marshal(query)
		q.Set("query", string(b))
	}
	if options != nil {
		b, _ := json.Marshal(options)
		q.Set("options", string(b))
	}
	data, err := c.do(ctx, http.MethodGet, "/api/v1/find-all/"+c.workspace, q, nil)
	if err != nil {
		return 0, err
	}
	var res struct {
		Value json.RawMessage `json:"value"`
		Total int             `json:"total"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return 0, fmt.Errorf("decode find-all: %w", err)
	}
	if out != nil && len(res.Value) > 0 {
		if err := json.Unmarshal(res.Value, out); err != nil {
			return 0, fmt.Errorf("decode find-all value: %w", err)
		}
	}
	return res.Total, nil
}

// FindOne returns the first match (limit 1). ok=false when none found.
func (c *RestClient) FindOne(ctx context.Context, class string, query map[string]any, out any) (bool, error) {
	// out must be *[]T; we pass through and report whether anything decoded.
	total, err := c.FindAll(ctx, class, query, map[string]any{"limit": 1}, out)
	if err != nil {
		return false, err
	}
	return total > 0, nil
}

// Tx posts a transaction.
func (c *RestClient) Tx(ctx context.Context, tx Tx) error {
	b, _ := json.Marshal(tx)
	_, err := c.do(ctx, http.MethodPost, "/api/v1/tx/"+c.workspace, nil, b)
	return err
}

// Account is the current account identity.
type Account struct {
	UUID string `json:"uuid"`
	Role string `json:"role"`
}

// GetAccount returns the authenticated account.
func (c *RestClient) GetAccount(ctx context.Context) (Account, error) {
	data, err := c.do(ctx, http.MethodGet, "/api/v1/account/"+c.workspace, nil, nil)
	if err != nil {
		return Account{}, err
	}
	var a Account
	if err := json.Unmarshal(data, &a); err != nil {
		return Account{}, fmt.Errorf("decode account: %w", err)
	}
	return a, nil
}
