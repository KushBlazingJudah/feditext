package fedi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/KushBlazingJudah/feditext/config"
)

var Proxy proxy
var ua = fmt.Sprintf("feditext/%s", config.Version)

type proxy struct {
	client http.Client
}

func NewProxy(proxyUrl string) (proxy, error) {
	if proxyUrl == "" {
		// Zero value is fine
		return proxy{}, nil
	}

	u, err := url.Parse(proxyUrl)
	if err != nil {
		return proxy{}, err
	}

	hc := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(u),
		},
		Timeout: config.RequestTimeout,
	}

	return proxy{client: hc}, nil
}

func (p proxy) Request(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", ua)

	return p.client.Do(req)
}

func (p proxy) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", ua)
	return p.client.Do(req)
}
