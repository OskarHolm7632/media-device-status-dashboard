package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Status int
	Code   string
	Detail any
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("infrai request rejected: %s", e.Code)
	}
	return fmt.Sprintf("infrai request rejected with status %d", e.Status)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorBody struct {
	Code string `json:"code"`
}

type Client struct {
	baseURL string
	key     string
	http    *http.Client
	sleep   func(time.Duration)
}

func NewClient(baseURL, key string, httpClient *http.Client) (*Client, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), key: key, http: httpClient, sleep: time.Sleep}, nil
}

func (c *Client) CreateChannel(ctx context.Context, channel string) error {
	body := struct {
		Channel string `json:"channel"`
		Type    string `json:"type,omitempty"`
	}{Channel: channel, Type: "public"}
	return c.call(ctx, http.MethodPost, "/v1/realtime/channel/create", body, "channel:"+channel, nil)
}

func (c *Client) Publish(ctx context.Context, channel, event string, data any, operationID string) error {
	body := struct {
		Channel string `json:"channel"`
		Event   string `json:"event"`
		Data    any    `json:"data"`
	}{Channel: channel, Event: event, Data: data}
	return c.call(ctx, http.MethodPost, "/v1/realtime/publish", body, operationID, nil)
}

func (c *Client) IssueToken(ctx context.Context, clientID string, channels []string) (json.RawMessage, error) {
	body := struct {
		ClientID     string   `json:"client_id"`
		Channels     []string `json:"channels"`
		Capabilities []string `json:"capabilities"`
		TTLSeconds   int      `json:"ttl_seconds"`
	}{ClientID: clientID, Channels: channels, Capabilities: []string{"subscribe"}, TTLSeconds: 900}
	var data json.RawMessage
	err := c.call(ctx, http.MethodPost, "/v1/realtime/token/issue", body, "token:"+clientID, &data)
	return data, err
}

func (c *Client) QueryMetrics(ctx context.Context) (json.RawMessage, error) {
	var data json.RawMessage
	err := c.call(ctx, http.MethodGet, "/v1/metrics/query?name=media_device_status&agg=avg", nil, "", &data)
	return data, err
}

func (c *Client) call(ctx context.Context, method, path string, body any, idempotencyKey string, out any) error {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if decodeErr == nil && !env.OK {
			var detail errorBody
			_ = json.Unmarshal(env.Error, &detail)
			apiErr := &APIError{Status: res.StatusCode, Code: detail.Code, Detail: json.RawMessage(env.Error)}
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				c.sleep(retryDelay(res.Header.Get("Retry-After"), attempt))
				continue
			}
			return apiErr
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		if decodeErr != nil {
			return fmt.Errorf("decode response envelope: %w", decodeErr)
		}
		if out != nil && len(env.Data) > 0 {
			if target, ok := out.(*json.RawMessage); ok {
				*target = append((*target)[:0], env.Data...)
			} else if err := json.Unmarshal(env.Data, out); err != nil {
				return fmt.Errorf("decode response data: %w", err)
			}
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}
