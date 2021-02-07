package service

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestProxy(t *testing.T) {
	rtConfig := map[string]*ServersTransport{
		"server1Config": {
			ServerName:          "server1",
			MaxIdleConnsPerHost: 100,
		},
	}
	rtManager := NewRoundTripperManager()
	rtManager.Update(rtConfig)
	rt, err := rtManager.Get("server1Config")
	if err != nil {
		t.Error(err)
		return
	}

	fullUrl := fmt.Sprintf("%s://%s", "http", "localhost:9090")
	parsedUrl, err := url.Parse(fullUrl)
	if err != nil {
		t.Error(err)
		return
	}

	proxy, err := buildProxy(parsedUrl, 100*time.Millisecond, rt, newBufferPool())
	if err != nil {
		t.Error(err)
		return
	}

	if err := http.ListenAndServe(":8881", proxy); err != nil {
		t.Error(err)
		return
	}
}
