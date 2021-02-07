package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"proxy/http_proxy"
	"time"
)

func main() {
	s := &http.Server{
		Addr:           fmt.Sprintf(":%d", 7071),
		Handler:        apiForwardHandler(),
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1024 * 1024,
	}

	log.Printf("api server listen and serve on :%d\n", 7071)
	log.Fatal(s.ListenAndServe())
}

func apiForwardHandler() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		apiForward(rw, req)
	}
}

func apiForward(rw http.ResponseWriter, req *http.Request) {
	fullUrl := fmt.Sprintf("%s://%s", "https", "server.tls.example.com:443/1")
	parsedUrl, err := url.Parse(fullUrl)
	if err != nil {
		panic(err)
	}
	proxy := http_proxy.NewSingleHostReverseProxy(parsedUrl)

	cert, err := tls.LoadX509KeyPair("./keys/client/client_cert.pem", "./keys/client/private/client_key.pem")
	if err != nil {
		log.Println(err)
		return
	}
	certBytes, err := ioutil.ReadFile("./keys/ca/ca_cert.pem")
	if err != nil {
		panic("Unable to read cert.pem")
	}
	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM(certBytes)
	if !ok {
		panic("failed to parse root certificate")
	}
	proxy.Transport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 0,
		}).DialContext,
		DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{
			RootCAs:      pool,
			Certificates: []tls.Certificate{cert},
		}}

	proxy.ServeHTTP(rw, req)
}
