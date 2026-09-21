// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// BuildTLSConfig returns a *tls.Config for the given provider TLS settings.
// Pass insecure=true to skip certificate verification.
// Pass caFile with a path to a PEM-encoded CA certificate to pin a custom CA.
// Pass both as zero values for system-default certificate verification.
func BuildTLSConfig(insecure bool, caFile string) (*tls.Config, error) {
	cfg := &tls.Config{}

	if insecure {
		cfg.InsecureSkipVerify = true
		return cfg, nil
	}

	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA certificate %s: %w", caFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("no valid certificates found in %s", caFile)
		}
		cfg.RootCAs = pool
	}

	return cfg, nil
}
