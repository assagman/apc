package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/assagman/apc/internal/logger"
)

type BaseHttpClient struct {
}

func New() *BaseHttpClient {
	return &BaseHttpClient{}
}

func (c *BaseHttpClient) Get(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	client := http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for hk, hv := range headers {
		req.Header.Add(hk, hv)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	return respBytes, nil
}

func (c *BaseHttpClient) Post(ctx context.Context, url string, headers map[string]string, body []byte) ([]byte, error) {
	// println("url: " + url)
	client := http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	for hk, hv := range headers {
		req.Header.Add(hk, hv)
	}

	// reqpb, err := httputil.DumpRequest(req, true)
	// if err != nil {
	// 	return nil, err
	// }
	// println(string(reqpb))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return nil, err
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		if resp.StatusCode == 429 { // too many requests
			sleepTime := 5 * time.Second
			logger.Warning("Request status code: %d. Retrying after %d", resp.StatusCode, sleepTime)
			time.Sleep(sleepTime)
			return c.Post(ctx, url, headers, body)
		}
		return respBytes, fmt.Errorf("Non-200 POST request. Status: %s.\nResponse dump:\n\n%s\n", resp.Status, string(respDump))
	}

	return respBytes, nil
}
