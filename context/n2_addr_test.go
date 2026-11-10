// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"net"
	"testing"

	"github.com/ishidawataru/sctp"
)

func TestParseN2Addrs(t *testing.T) {
	const (
		gnbIPv4 = "10.10.2.232"
		gnbIPv6 = "2001:db8:620::232"
	)
	sctpAddr := func(port int, ips ...string) string {
		addr := &sctp.SCTPAddr{Port: port}
		for _, ip := range ips {
			addr.IPAddrs = append(addr.IPAddrs, net.IPAddr{IP: net.ParseIP(ip)})
		}
		return addr.String()
	}
	tests := []struct {
		name     string
		remote   string
		wantIPv4 string
		wantIPv6 string
	}{
		// The forms sctp.SCTPAddr.String() produces, i.e. what GnbIp holds.
		{name: "IPv4 association", remote: sctpAddr(38412, gnbIPv4), wantIPv4: gnbIPv4},
		{name: "IPv6 association", remote: sctpAddr(38412, gnbIPv6), wantIPv6: gnbIPv6},
		{name: "multi-homed IPv4", remote: sctpAddr(38412, gnbIPv4, "10.10.3.232"), wantIPv4: gnbIPv4},
		{name: "multi-homed IPv6", remote: sctpAddr(38412, gnbIPv6, "2001:db8:630::232"), wantIPv6: gnbIPv6},
		{name: "multi-homed dual stack", remote: sctpAddr(38412, gnbIPv6, gnbIPv4), wantIPv4: gnbIPv4, wantIPv6: gnbIPv6},
		{name: "IPv4-mapped IPv6", remote: sctpAddr(38412, "::ffff:"+gnbIPv4), wantIPv4: gnbIPv4},
		// Other spellings.
		{name: "literal IPv4 string", remote: "10.10.2.232:38412", wantIPv4: gnbIPv4},
		{name: "literal IPv6 string", remote: "[2001:db8:620::232]:38412", wantIPv6: gnbIPv6},
		{name: "IPv6 not in RFC 5952 form", remote: "[2001:DB8:620:0::232]:38412", wantIPv6: gnbIPv6},
		{name: "bare IPv4", remote: gnbIPv4, wantIPv4: gnbIPv4},
		{name: "bare IPv6", remote: gnbIPv6, wantIPv6: gnbIPv6},
		{name: "bracketed IPv6 without port", remote: "[" + gnbIPv6 + "]", wantIPv6: gnbIPv6},
		{name: "zoned IPv6 is skipped", remote: "[fe80::1%eth0]/10.10.2.232:38412", wantIPv4: gnbIPv4},
		{name: "empty", remote: ""},
		{name: "host name", remote: "gnb.example:38412"},
		{name: "garbage", remote: "[::1:38412"},
		{name: "port only", remote: ":38412"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ran := &AmfRan{GnbIp: test.remote}
			ipv4, ipv6 := ran.N2Addrs()
			if ipv4 != test.wantIPv4 || ipv6 != test.wantIPv6 {
				t.Fatalf("N2Addrs() for %q = (%q, %q), want (%q, %q)", test.remote, ipv4, ipv6, test.wantIPv4, test.wantIPv6)
			}
		})
	}
}
