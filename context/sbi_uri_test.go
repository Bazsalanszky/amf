// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/util/sbiaddr"
)

func TestGetSbiUri(t *testing.T) {
	const (
		ipv4    = "10.42.0.5"
		ipv6    = "2001:db8::5"
		ipv4URI = "https://" + ipv4 + ":29518"
		ipv6URI = "https://[" + ipv6 + "]:29518"
	)
	tests := []struct {
		name         string
		registerIPv4 string
		registerIPv6 string
		pref         sbiaddr.Preference
		want         string
	}{
		{name: "IPv4", registerIPv4: ipv4, want: ipv4URI},
		{name: "FQDN", registerIPv4: "amf.sdcore.svc", want: "https://amf.sdcore.svc:29518"},
		{name: "IPv6 in RegisterIPv4", registerIPv4: ipv6, want: ipv6URI},
		{name: "IPv6 only", registerIPv6: ipv6, want: ipv6URI},
		{name: "dual, IPv4 preferred", registerIPv4: ipv4, registerIPv6: ipv6, want: ipv4URI},
		{
			name: "dual, IPv6 preferred", registerIPv4: ipv4, registerIPv6: ipv6,
			pref: sbiaddr.PreferIPv6, want: ipv6URI,
		},
		{
			name: "FQDN wins over the preference", registerIPv4: "amf", registerIPv6: ipv6,
			pref: sbiaddr.PreferIPv6, want: "https://amf:29518",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := &AMFContext{
				UriScheme: models.URISCHEME_HTTPS, SBIPort: 29518,
				RegisterIPv4: test.registerIPv4, RegisterIPv6: test.registerIPv6, SbiPreference: test.pref,
			}
			got := c.GetSbiUri()
			if got != test.want {
				t.Fatalf("GetSbiUri() = %q, want %q", got, test.want)
			}
			if c.GetIPv4Uri() != got {
				t.Fatalf("GetIPv4Uri() = %q, want alias of %q", c.GetIPv4Uri(), got)
			}
			u, err := url.Parse(got)
			if err != nil || u.Port() != "29518" {
				t.Fatalf("url.Parse(%q) = %v, %v; want port 29518", got, u, err)
			}
		})
	}
}

// For an IPv4 address, a host name and the values the old code accepted
// unchecked, GetSbiUri is byte-identical to the old fmt.Sprintf.
func TestGetSbiUriKeepsLegacyFormat(t *testing.T) {
	hosts := []string{"127.0.0.18", "10.42.0.5", "amf", "amf.sdcore.svc.cluster.local", "", "not a host"}
	schemes := []models.UriScheme{models.URISCHEME_HTTP, models.URISCHEME_HTTPS, ""}
	for _, host := range hosts {
		for _, scheme := range schemes {
			for _, port := range []int{0, 80, 8000, 29518} {
				c := &AMFContext{UriScheme: scheme, SBIPort: port, RegisterIPv4: host}
				want := fmt.Sprintf("%s://%s:%d", scheme, host, port)
				if got := c.GetSbiUri(); got != want {
					t.Errorf("GetSbiUri() with %q, %q, %d = %q, want %q", scheme, host, port, got, want)
				}
			}
		}
	}
}
