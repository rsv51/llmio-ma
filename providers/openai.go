package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tidwall/sjson"
)

type OpenAI struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

// GetHost 获取OpenAI的主机地址
func (o *OpenAI) GetHost() string {
	return o.BaseURL
}

// GetTimeout 获取请求超时时间
func (o *OpenAI) GetTimeout() time.Duration {
	return 30 * time.Second
}

func (o *OpenAI) Chat(ctx context.Context, client *http.Client, model string, rawBody []byte) (*http.Response, error) {
	body, err := sjson.SetBytes(rawBody, "model", model)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", o.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.APIKey))

	return client.Do(req)
}

func (o *OpenAI) Models(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/models", o.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.APIKey))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", res.StatusCode)
	}

	var modelList ModelList
	if err := json.NewDecoder(res.Body).Decode(&modelList); err != nil {
		return nil, err
	}
	return modelList.Data, nil
}
