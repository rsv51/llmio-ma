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

type Anthropic struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Version string `json:"version"`
	Beta    string `json:"beta"`
}

// GetHost 获取Anthropic的主机地址
func (a *Anthropic) GetHost() string {
	return a.BaseURL
}

// GetTimeout 获取请求超时时间
func (a *Anthropic) GetTimeout() time.Duration {
	return 30 * time.Second
}

func (a *Anthropic) Chat(ctx context.Context, client *http.Client, model string, rawBody []byte) (*http.Response, error) {
	body, err := sjson.SetBytes(rawBody, "model", model)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/messages", a.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", a.Version)
	req.Header.Set("anthropic-beta", a.Beta)
	return client.Do(req)
}

type AnthropicModelsResponse struct {
	Data    []AnthropicModel `json:"data"`
	FirstID string           `json:"first_id"`
	HasMore bool             `json:"has_more"`
	LastID  string           `json:"last_id"`
}

type AnthropicModel struct {
	CreatedAt   time.Time `json:"created_at"`
	DisplayName string    `json:"display_name"`
	ID          string    `json:"id"`
	Type        string    `json:"type"`
}

func (a *Anthropic) Models(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/models", a.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", a.Version)
	req.Header.Set("anthropic-beta", a.Beta)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", res.StatusCode)
	}
	var anthropicModels AnthropicModelsResponse
	if err := json.NewDecoder(res.Body).Decode(&anthropicModels); err != nil {
		return nil, err
	}

	var modelList ModelList
	for _, model := range anthropicModels.Data {
		modelList.Data = append(modelList.Data, Model{
			ID:      model.ID,
			Created: model.CreatedAt.Unix(),
		})
	}
	return modelList.Data, nil
}
