// Copyright 2015 CoreOS, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"regexp"

	log "github.com/Sirupsen/logrus"
	"github.com/quentin-m/goproxy"
)

const httpRegexp = `^.*:80$`

type Handler func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response)

func NewProxy(proxyHandler Handler, caKeyPath, caCertPath string) (*goproxy.ProxyHttpServer, error) {
	// Initialize the forward proxy's MITM handler using the specified CA key pair.
	var mitmHandler goproxy.FuncHttpsHandler
	if caKeyPath == "" || caCertPath == "" {
		log.Warning("No CA keypair specified, the proxy will not be able to forward requests to TLS endpoints.")
	} else {
		var err error
		mitmHandler, err = setupMITMHandler(caKeyPath, caCertPath)
		if err != nil {
			return nil, err
		}
	}

	// Create a forward proxy.
	proxy := goproxy.NewProxyHttpServer()
	proxy.Tr = &http.Transport{}
	proxy.Verbose = log.GetLevel() == log.DebugLevel

	// Handle HTTP requests with the specified handler.
	p := proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile(httpRegexp)))
	p.DoFunc(proxyHandler)

	// Handle HTTPs requests with MITM and the specified handler.
	p = proxy.OnRequest(goproxy.Not(goproxy.ReqHostMatches(regexp.MustCompile(httpRegexp))))
	if mitmHandler != nil {
		p.HandleConnect(mitmHandler)
	}
	p.DoFunc(proxyHandler)

	return proxy, nil
}

func NewReverseProxy(proxyHandler Handler) (*goproxy.ProxyHttpServer, error) {
	// Create a reverse proxy.
	reverseProxy := goproxy.NewReverseProxyHttpServer()
	reverseProxy.Tr = &http.Transport{}
	reverseProxy.Verbose = log.GetLevel() == log.DebugLevel

	// Handle requests with the specified handler.
	reverseProxy.OnRequest().DoFunc(proxyHandler)

	return reverseProxy, nil
}

func setupMITMHandler(caKeyPath, caCertPath string) (goproxy.FuncHttpsHandler, error) {
	ca, err := readCA(caKeyPath, caCertPath)
	if err != nil {
		return nil, err
	}

	return func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		return &goproxy.ConnectAction{
			Action:    goproxy.ConnectMitm,
			TLSConfig: goproxy.TLSConfigFromCA(ca),
		}, host
	}, nil
}

func readCA(caKeyPath, caCertPath string) (*tls.Certificate, error) {
	caCert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		return nil, err
	}

	caKey, err := ioutil.ReadFile(caKeyPath)
	if err != nil {
		return nil, err
	}

	ca, err := tls.X509KeyPair(caCert, caKey)
	if err != nil {
		return nil, err
	}

	ca.Leaf, err = x509.ParseCertificate(ca.Certificate[0])
	return &ca, err
}
