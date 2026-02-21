package net

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
)

func PrintHTTPResponse(resp *http.Response) {
	if resp == nil {
		return
	}
	if respDump, err := httputil.DumpResponse(resp, true); err == nil {
		slog.Debug("http response", "body", string(respDump))
	}
}
