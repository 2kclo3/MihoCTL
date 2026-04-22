package mihomo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"mihoctl/internal/core"
)

type Client struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

type ProxyGroup struct {
	Name string   `json:"name"`
	Type string   `json:"type"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
}

type proxyResponse struct {
	Proxies map[string]ProxyGroup `json:"proxies"`
}

type configResponse struct {
	Version string `json:"version"`
	Tun     struct {
		Enable bool `json:"enable"`
	} `json:"tun"`
}

func NewClient(baseURL, secret string) *Client {
	return NewClientWithHTTPClient(baseURL, secret, &http.Client{
		Timeout: 15 * time.Second,
	})
}

func NewClientWithHTTPClient(baseURL, secret string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 15 * time.Second,
		}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		secret:     secret,
		httpClient: httpClient,
	}
}

func (c *Client) Ping(ctx context.Context) (string, error) {
	var payload struct {
		Version string `json:"version"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/version", nil, &payload); err != nil {
		return "", err
	}
	return payload.Version, nil
}

func (c *Client) ListProxyGroups(ctx context.Context) ([]ProxyGroup, error) {
	var payload proxyResponse
	if err := c.doJSON(ctx, http.MethodGet, "/proxies", nil, &payload); err != nil {
		return nil, err
	}

	groups := make([]ProxyGroup, 0, len(payload.Proxies))
	for _, group := range payload.Proxies {
		if len(group.All) == 0 {
			continue
		}
		sort.Strings(group.All)
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})
	return groups, nil
}

func (c *Client) UseProxy(ctx context.Context, group, proxy string) error {
	body := map[string]string{"name": proxy}
	return c.doJSON(ctx, http.MethodPut, "/proxies/"+url.PathEscape(group), body, nil)
}

func (c *Client) CheckGroupDelay(ctx context.Context, group, testURL string, timeoutMS int) (map[string]int, error) {
	query := url.Values{}
	query.Set("url", testURL)
	query.Set("timeout", fmt.Sprintf("%d", timeoutMS))

	var payload map[string]int
	endpoint := path.Join("/group", url.PathEscape(group), "delay") + "?" + query.Encode()
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) CheckProxyDelay(ctx context.Context, proxy, testURL string, timeoutMS int) (int, error) {
	query := url.Values{}
	query.Set("url", testURL)
	query.Set("timeout", fmt.Sprintf("%d", timeoutMS))

	var payload struct {
		Delay int `json:"delay"`
	}
	endpoint := path.Join("/proxies", url.PathEscape(proxy), "delay") + "?" + query.Encode()
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &payload); err != nil {
		return 0, err
	}
	return payload.Delay, nil
}

func (c *Client) ReloadConfig(ctx context.Context, configPath string) error {
	query := url.Values{}
	query.Set("force", "true")
	endpoint := "/configs?" + query.Encode()
	body := map[string]string{"path": configPath}
	return c.doJSON(ctx, http.MethodPut, endpoint, body, nil)
}

func (c *Client) SetTun(ctx context.Context, enabled bool) error {
	body := map[string]any{
		"tun": map[string]bool{
			"enable": enabled,
		},
	}
	return c.doJSON(ctx, http.MethodPatch, "/configs", body, nil)
}

func (c *Client) GetConfig(ctx context.Context) (*configResponse, error) {
	payload := &configResponse{}
	if err := c.doJSON(ctx, http.MethodGet, "/configs", nil, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return core.NewActionError("controller_request_failed", "err.http.request", err, "err.http.check_controller", map[string]any{
			"addr": c.baseURL,
		}, nil)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return core.NewActionError("controller_status_failed", "err.http.status", fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))), "err.http.check_controller", map[string]any{
			"addr": c.baseURL,
		}, nil)
	}

	if target == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return core.NewActionError("controller_decode_failed", "err.http.request", err, "err.http.check_controller", map[string]any{
			"addr": c.baseURL,
		}, nil)
	}
	return nil
}
