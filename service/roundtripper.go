package service

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"
)

type RoundTripperManager struct {
	rwLock          sync.RWMutex
	roundTripperMap map[string]http.RoundTripper
	configMap       map[string]*ServersTransport
}

func NewRoundTripperManager() *RoundTripperManager {
	return &RoundTripperManager{
		rwLock:          sync.RWMutex{},
		roundTripperMap: make(map[string]http.RoundTripper),
		configMap:       make(map[string]*ServersTransport),
	}
}

func (rt *RoundTripperManager) Update(newConfigMap map[string]*ServersTransport) {
	rt.rwLock.Lock()
	defer rt.rwLock.Unlock()

	for configName, newConfig := range newConfigMap {
		oldConfig, ok := rt.configMap[configName]
		if !ok {
			delete(rt.configMap, configName)
			delete(rt.roundTripperMap, configName)
			continue
		}

		if reflect.DeepEqual(newConfig, oldConfig) {
			break
		}

		roundTripper, err := createRoundTripper(newConfig)
		if err != nil {
			logrus.Errorf("create round tripper error:%s, use default transport", err.Error())
			rt.roundTripperMap[configName] = http.DefaultTransport
			continue
		}

		rt.roundTripperMap[configName] = roundTripper
	}

	for newConfigName, newConfig := range newConfigMap {
		if _, ok := rt.configMap[newConfigName]; ok {
			continue
		}

		roundTripper, err := createRoundTripper(newConfig)
		if err != nil {
			logrus.Errorf("create round tripper error:%s, use default transport", err)
			rt.roundTripperMap[newConfigName] = http.DefaultTransport
		}

		rt.roundTripperMap[newConfigName] = roundTripper
	}

	rt.configMap = newConfigMap
}

func (rt *RoundTripperManager) Get(name string) (http.RoundTripper, error) {
	if len(name) == 0 {
		return nil, errors.New("the name can not be empty")
	}

	rt.rwLock.RLock()
	defer rt.rwLock.RUnlock()

	if rt, ok := rt.roundTripperMap[name]; ok {
		return rt, nil
	}

	return nil, fmt.Errorf("servers transport not found %s", name)
}

func createRoundTripper(config *ServersTransport) (http.RoundTripper, error) {
	if config == nil {
		return nil, errors.New("server transport config is nil")
	}
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	if config.ForwardingTimeouts != nil {
		dialer.Timeout = config.ForwardingTimeouts.DialTimeout
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		IdleConnTimeout:       60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if config.ForwardingTimeouts != nil {
		transport.ResponseHeaderTimeout = config.ForwardingTimeouts.ResponseHeaderTimeout
		transport.IdleConnTimeout = config.ForwardingTimeouts.IdleConnTimeout
	}

	if config.InsecureSkipVerify || len(config.RootCAs) > 0 || len(config.Certificates) > 0 {
		var certificates []tls.Certificate
		for i := range config.Certificates {
			c, err := config.Certificates[i].GetCertificate()
			if err != nil {
				logrus.Errorf("get certificate err:%s", err.Error())
				continue
			}
			certificates = append(certificates, c)
		}

		transport.TLSClientConfig = &tls.Config{
			ServerName:         config.ServerName,
			InsecureSkipVerify: config.InsecureSkipVerify,
			RootCAs:            createRootCACertPool(config.RootCAs),
			Certificates:       certificates,
		}
	}

	return newSmartRoundTripper(transport)

}

func createRootCACertPool(rootCAs []FileOrContent) *x509.CertPool {
	if len(rootCAs) == 0 {
		return nil
	}

	roots := x509.NewCertPool()

	for _, cert := range rootCAs {
		certContent, err := cert.Read()
		if err != nil {
			logrus.Errorf("Error while read RootCAs err:%s", err.Error())
			continue
		}
		roots.AppendCertsFromPEM(certContent)
	}

	return roots
}
