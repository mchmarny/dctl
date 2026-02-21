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
	respDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		slog.Debug("error dumping http response", "error", err)
		return
	}
	slog.Debug("http response", "body", string(respDump))
}
