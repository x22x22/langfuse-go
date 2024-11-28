package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	langfuseDefaultEndpoint = "https://cloud.langfuse.com"
)

var (
	once     sync.Once
	instance *Client
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	auth       string
}

// GetInstance 返回 Client 的单例实例
func GetInstance() *Client {
	once.Do(func() {
		instance = newClient()
	})
	return instance
}

func newClient() *Client {
	langfuseHost := os.Getenv("LANGFUSE_HOST")
	if langfuseHost == "" {
		langfuseHost = langfuseDefaultEndpoint
	}

	publicKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	secretKey := os.Getenv("LANGFUSE_SECRET_KEY")

	// 创建优化的传输层配置
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,              // 连接池最大空闲连接数
		MaxIdleConnsPerHost:   100,              // 每个主机的最大空闲连接数
		MaxConnsPerHost:       0,                // 每个主机的最大连接数，0表示无限制
		IdleConnTimeout:       90 * time.Second, // 空闲连接超时时间
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false, // 启用 keep-alive
		// 启用连接复用的关键配置
		ResponseHeaderTimeout: 30 * time.Second, // 响应头超时
		DisableCompression:    false,            // 启用压缩
	}

	// 创建 HTTP 客户端
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   time.Second * 30,
	}

	return &Client{
		baseURL:    langfuseHost,
		httpClient: httpClient,
		auth:       basicAuth(publicKey, secretKey),
	}
}

func (c *Client) Ingestion(ctx context.Context, req *Ingestion, res *IngestionResponse) error {
	// 打印 httpClient 的内存地址

	// 序列化请求体
	jsonData, err := json.Marshal(req)
	if err != nil {
		return err
	}

	// 创建请求
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/api/public/ingestion",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		return err
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", c.auth)

	// 发送请求
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 设置响应状态码
	res.Code = resp.StatusCode

	// 设置原始响应体
	rawBody := string(body)
	res.RawBody = &rawBody

	// 解析响应体
	return json.Unmarshal(body, res)
}

func basicAuth(publicKey, secretKey string) string {
	auth := publicKey + ":" + secretKey
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}
