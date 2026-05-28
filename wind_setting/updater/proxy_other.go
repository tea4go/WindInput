//go:build !windows

package updater

import (
	"net/http"
	"net/url"
)

// systemProxyURL 非 Windows 平台无注册表代理; 统一返回 nil, 走环境变量代理。
func systemProxyURL() *url.URL { return nil }

// newHTTPClient 返回走环境变量代理 (HTTP_PROXY/HTTPS_PROXY) 的 http.Client。
func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
	}
}
