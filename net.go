package main

import (
	"net/http"
	"net/url"
	"time"
)

var proxyURL = func() *url.URL {
	u, err := url.Parse("socks5://localhost:10000")
	ce(err)
	return u
}()

var proxyHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	},
	Timeout: time.Second * 30,
}
