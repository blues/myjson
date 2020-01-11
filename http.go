// Copyright Blues Inc.  All rights reserved. 
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Common support for all HTTP topic handlers
package main

import (
	"fmt"
	"strings"
    "net/http"
	"net/url"
    "io/ioutil"
    "crypto/tls"
    "crypto/x509"
)

// HTTPInboundHandler kicks off inbound messages coming from all sources, then serve HTTP
func HTTPInboundHandler(port string, portSecure string) {

	// Topics
    http.HandleFunc("/github", inboundWebGithubHandler)
    http.HandleFunc("/ping", inboundWebPingHandler)

	// HTTP
    fmt.Printf("Now handling inbound HTTP on %s\n", port)
    go http.ListenAndServe(port, nil)

	// HTTPS
    fmt.Printf("Now handling inbound HTTPS on %s\n", portSecure)

	// Build TLS context for bidirectional authentication
    httpsClientCertPool := x509.NewCertPool()

    httpsClientCA, err := ioutil.ReadFile(configCertDirectory + "certifier.pem")
    if err != nil {
        fmt.Printf("Error opening service certificate: %v\n", err)
        return
    }
    httpsClientCertPool.AppendCertsFromPEM(httpsClientCA)

	// Build from cert pool
    tlsConfig := &tls.Config{
        ClientCAs: httpsClientCertPool,
        ClientAuth: tls.RequireAndVerifyClientCert,
    }
    tlsConfig.BuildNameToCertificate()

	// Serve TLS
	server := &http.Server {
		Addr: portSecure,
		TLSConfig: tlsConfig,
	}
    server.ListenAndServeTLS(configCertDirectory + "public.pem", configCertDirectory + "private.pem")

}

// HTTPArgs parses the request URI and returns interesting things
func HTTPArgs(req *http.Request, topic string) (target string, args map[string]string) {
	args = map[string]string{}

	// Trim the request URI
	target = req.RequestURI[len(topic):]

	// If nothing left, there were no args
	if len(target) == 0 {
		return
	}

	// Make sure that the prefix is "/", else the pattern matcher is matching something we don't want
	if strings.HasPrefix(target, "/") {
		target = strings.TrimPrefix(target, "/")
	}

	// See if there is a query, and if so process it
	str := strings.SplitN(target, "?", 2)
	if len(str) == 1 {
		return
	}

	// Now that we know we have args, parse them
	target = str[0]
	values, err := url.ParseQuery(str[1])
	if err != nil {
		return
	}

	// Generate the return arg in the format we expect
	for k, v := range(values) {
		if len(v) == 1 {
			args[k] = strings.TrimSuffix(strings.TrimPrefix(v[0], "\""), "\"")
		}
	}

	// Done
	return 
	
}
