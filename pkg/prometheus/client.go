package prometheus

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// Client 封装 Prometheus 客户端
type Client struct {
	API v1.API
	URL string
}

// basicAuthRoundTripper 实现 Basic Auth 鉴权
type basicAuthRoundTripper struct {
	username string
	password string
	next     http.RoundTripper
}

func (b *basicAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// 创建 Basic Auth 头
	auth := b.username + ":" + b.password
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))

	// 添加 Authorization 头
	req.Header.Set("Authorization", "Basic "+encodedAuth)

	return b.next.RoundTrip(req)
}

// HealthCheck 测试 Prometheus 连通性（执行空查询 query=1）
func (c *Client) HealthCheck(ctx context.Context) error {
	log.Printf("[HealthCheck] 测试数据源连接: %s", c.URL)
	_, _, err := c.API.Query(ctx, "1", time.Now())
	if err != nil {
		log.Printf("[HealthCheck] 连接失败: %s - %v", c.URL, err)
	} else {
		log.Printf("[HealthCheck] 连接成功: %s", c.URL)
	}
	return err
}

// NewClient 创建新的 Prometheus 客户端
func NewClient(url, username, password string) (*Client, error) {
	httpClient := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}

	config := api.Config{
		Address: url,
		Client:  httpClient,
	}
	if username != "" && password != "" {
		rt := &basicAuthRoundTripper{
			username: username,
			password: password,
			next:     httpClient.Transport,
		}
		httpClient.Transport = rt
		config.Client = httpClient
	}
	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("creating prometheus client: %w", err)
	}

	return &Client{
		API: v1.NewAPI(client),
		URL: url,
	}, nil
}
