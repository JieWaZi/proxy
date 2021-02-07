package service

import (
	"golang.org/x/net/http/httpguts"
	"golang.org/x/net/http2"
	"net/http"
)

type smartRoundTripper struct {
	http  *http.Transport
	http2 *http.Transport
}

func newSmartRoundTripper(transport *http.Transport) (http.RoundTripper, error) {
	httpTransport := transport.Clone()
	err := http2.ConfigureTransport(transport)
	if err != nil {
		return nil, err
	}

	return &smartRoundTripper{
		http:  httpTransport,
		http2: transport,
	}, nil
}

func (m *smartRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if httpguts.HeaderValuesContainsToken(req.Header["Connection"], "Upgrade") {
		return m.http.RoundTrip(req)
	}

	return m.http2.RoundTrip(req)
}
