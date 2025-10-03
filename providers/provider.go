package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"` // 使用 int64 存储 Unix 时间戳
	OwnedBy string `json:"owned_by"`
}

type Provider interface {
	// client 用于HTTP请求的客户端
	Chat(ctx context.Context, client *http.Client, model string, rawData []byte) (*http.Response, error)
	Models(ctx context.Context) ([]Model, error)
}

// PooledProvider 支持连接池的Provider接口
type PooledProvider interface {
	Provider
	GetHost() string // 获取Provider的主机地址
	GetTimeout() time.Duration // 获取请求超时时间
}

func New(Type, providerConfig string) (Provider, error) {
	switch Type {
	case "openai":
		var openai OpenAI
		if err := json.Unmarshal([]byte(providerConfig), &openai); err != nil {
			return nil, err
		}
		// 返回支持连接池的包装器
		return NewPooledProviderWrapper(&openai, openai.BaseURL, 30*time.Second), nil
	case "anthropic":
		var anthropic Anthropic
		if err := json.Unmarshal([]byte(providerConfig), &anthropic); err != nil {
			return nil, err
		}
		// 返回支持连接池的包装器
		return NewPooledProviderWrapper(&anthropic, anthropic.BaseURL, 30*time.Second), nil
	default:
		return nil, errors.New("unknown provider type")
	}
}
