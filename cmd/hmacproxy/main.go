// Copyright 2016 CoreOS, Inc
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

package main

import (
	"flag"
	"net/http"
	"net/http/httptest"
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/coreos-inc/hmacproxy"
	"github.com/coreos-inc/hmacproxy/config"
	"github.com/coreos-inc/hmacproxy/credential"
	_ "github.com/coreos-inc/hmacproxy/credential/singlekey"
)

func main() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagConfigPath := flag.String("config", "", "Load configuration from the specified yaml file.")
	flagLogLevel := flag.String("log-level", "info", "Define the logging level.")
	flag.Parse()

	// Load configuration
	proxyConfig, err := config.Load(*flagConfigPath)
	if err != nil {
		flag.Usage()
		log.Fatalf("failed to load configuration: %s", err)
	}

	// Initialize logging system
	level, err := log.ParseLevel(*flagLogLevel)
	if err != nil {
		log.Fatal("failed to parse the log level")
	}
	log.SetLevel(level)

	if proxyConfig.Signer != nil {
		log.Infof("Starting signing proxy on: %s", proxyConfig.Signer.ListenerAddr)

		signingCredential := credential.Credential{
			proxyConfig.Signer.Key.ID,
			proxyConfig.Signer.Key.Secret,
			proxyConfig.Signer.Key.Service,
			proxyConfig.Signer.Key.Region,
		}

		signingProxy, err := hmacproxy.NewSigningProxy(signingCredential)
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal(http.ListenAndServe(proxyConfig.Signer.ListenerAddr, signingProxy))
	}

	if proxyConfig.Verifier != nil {
		log.Infof(
			"Starting verification proxy listening on: %s with upstream: %v",
			proxyConfig.Verifier.ListenerAddr,
			proxyConfig.Verifier.Upstream,
		)

		cs, err := credential.NewStore(proxyConfig.Verifier.CredentialSource)
		if err != nil {
			log.Fatal(err)
		}

		verificationProxy, err := hmacproxy.NewVerifyingProxy(proxyConfig.Verifier.Upstream.URL, cs)
		if err != nil {
			log.Fatal(err)
		}
		verificationServer := httptest.NewServer(verificationProxy)
		defer verificationServer.Close()
	}
}
