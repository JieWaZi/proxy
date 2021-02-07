package service

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// StatusClientClosedRequest non-standard HTTP status code for client disconnection.
const StatusClientClosedRequest = 499

// StatusClientClosedRequestText non-standard HTTP status for client disconnection.
const StatusClientClosedRequestText = "Client Closed Request"

func buildProxy(targetURL *url.URL, flushInterval time.Duration, roundTripper http.RoundTripper, bufferPool httputil.BufferPool) (http.Handler, error) {
	targetQuery := targetURL.RawQuery
	proxy := &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			request.URL.Scheme = targetURL.Scheme
			request.URL.Host = targetURL.Host
			request.URL.Path = singleJoiningSlash(targetURL.Path, request.URL.Path)
			if targetQuery == "" || request.URL.RawQuery == "" {
				request.URL.RawQuery = targetQuery + request.URL.RawQuery
			} else {
				request.URL.RawQuery = targetQuery + "&" + request.URL.RawQuery
			}
			request.RequestURI = "" // Outgoing request should not have RequestURI

			if _, ok := request.Header["User-Agent"]; !ok {
				request.Header.Set("User-Agent", "")
			}

		},
		Transport:     roundTripper,
		FlushInterval: flushInterval,
		BufferPool:    bufferPool,
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
			statusCode := http.StatusInternalServerError

			switch {
			case errors.Is(err, io.EOF):
				statusCode = http.StatusBadGateway
			case errors.Is(err, context.Canceled):
				statusCode = StatusClientClosedRequest
			default:
				var netErr net.Error
				if errors.As(err, &netErr) {
					if netErr.Timeout() {
						statusCode = http.StatusGatewayTimeout
					} else {
						statusCode = http.StatusBadGateway
					}
				}
			}

			logrus.Debugf("'%d %s' caused by: %v", statusCode, statusText(statusCode), err)
			writer.WriteHeader(statusCode)
			_, writerErr := writer.Write([]byte(statusText(statusCode)))
			if writerErr != nil {
				logrus.Debugf("writing status code err:%s", writerErr.Error())
			}
		},
	}

	return proxy, nil
}

func statusText(statusCode int) string {
	if statusCode == StatusClientClosedRequest {
		return StatusClientClosedRequestText
	}
	return http.StatusText(statusCode)
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
