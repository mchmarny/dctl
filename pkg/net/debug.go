package net

import (
	"net/http"
	"net/http/httputil"

	log "github.com/sirupsen/logrus"
)

func PrintHTTPResponse(resp *http.Response) {
	if respDump, err := httputil.DumpResponse(resp, true); err == nil {
		log.Debugf("%s", respDump)
	}
}
