// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	gocontext "context"
	"errors"
	"net"
	"slices"
	"testing"

	"github.com/ishidawataru/sctp"
)

const (
	testN2IPv4     = "10.10.2.18"
	testN2IPv6     = "2001:db8:620::18"
	testLoopback4  = "127.0.0.1"
	testWildcard4  = "0.0.0.0"
	testLoopback6  = "::1"
	testSecondIPv4 = "10.10.3.18"
)

func TestNgapListenAddrs(t *testing.T) {
	const dualName, v4Name, v6Name = "amf-n2.example", "amf-v4.example", "amf-v6.example"
	zone := net.IPAddr{IP: net.ParseIP("fe80::1"), Zone: "eth1"}
	resolver := map[string][]net.IPAddr{
		// The resolver sorts IPv6 first (RFC 6724); the old code still picked IPv4.
		dualName: {{IP: net.ParseIP(testN2IPv6)}, {IP: net.ParseIP(testN2IPv4)}},
		v4Name:   {{IP: net.ParseIP(testN2IPv4)}},
		v6Name:   {{IP: net.ParseIP(testN2IPv6)}},
		"multi":  {{IP: net.ParseIP(testN2IPv4)}, {IP: net.ParseIP(testSecondIPv4)}},
	}
	original := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = original })
	lookupIPAddr = func(_ gocontext.Context, host string) ([]net.IPAddr, error) {
		if addrs, ok := resolver[host]; ok {
			return addrs, nil
		}
		return nil, errors.New("no such host")
	}

	tests := []struct {
		name       string
		addresses  []string
		wantAll    []string
		wantLegacy []string
	}{
		{name: "chart default", addresses: []string{testWildcard4}, wantAll: []string{testWildcard4}, wantLegacy: []string{testWildcard4}},
		{name: "AMF default", addresses: []string{testLoopback4}, wantAll: []string{testLoopback4}, wantLegacy: []string{testLoopback4}},
		{name: "IPv6 wildcard", addresses: []string{"::"}, wantAll: []string{"::"}, wantLegacy: []string{"::"}},
		{name: "bracketed IPv6", addresses: []string{"[" + testN2IPv6 + "]"}, wantAll: []string{testN2IPv6}, wantLegacy: []string{testN2IPv6}},
		{name: "zoned IPv6", addresses: []string{"fe80::1%eth1"}, wantAll: []string{zone.String()}, wantLegacy: []string{zone.String()}},
		{name: "two literals", addresses: []string{testN2IPv4, testN2IPv6}, wantAll: []string{testN2IPv4, testN2IPv6}, wantLegacy: []string{testN2IPv4, testN2IPv6}},
		{name: "empty entry", addresses: []string{""}, wantAll: []string{""}, wantLegacy: []string{""}},
		{
			// Before: only 10.10.2.18 was bound.
			name: "dual-stack name binds both families", addresses: []string{dualName},
			wantAll: []string{testN2IPv4, testN2IPv6}, wantLegacy: []string{testN2IPv4},
		},
		{name: "IPv4 name", addresses: []string{v4Name}, wantAll: []string{testN2IPv4}, wantLegacy: []string{testN2IPv4}},
		{name: "IPv6 name", addresses: []string{v6Name}, wantAll: []string{testN2IPv6}, wantLegacy: []string{testN2IPv6}},
		{name: "name with two IPv4 addresses", addresses: []string{"multi"}, wantAll: []string{testN2IPv4, testSecondIPv4}, wantLegacy: []string{testN2IPv4}},
		{name: "duplicates dropped", addresses: []string{testN2IPv4, v4Name, dualName}, wantAll: []string{testN2IPv4, testN2IPv6}, wantLegacy: []string{testN2IPv4}},
		{name: "unresolvable name skipped", addresses: []string{"nowhere", testN2IPv4}, wantAll: []string{testN2IPv4}, wantLegacy: []string{testN2IPv4}},
		{name: "IPv4 with a zone is a name", addresses: []string{testN2IPv4 + "%eth1"}},
	}
	str := func(addrs []net.IPAddr) []string {
		var out []string
		for _, a := range addrs {
			out = append(out, a.String())
		}
		return out
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			all, legacy := ngapListenAddrs(test.addresses)
			if got := str(all); !slices.Equal(got, test.wantAll) {
				t.Errorf("all = %q, want %q", got, test.wantAll)
			}
			if got := str(legacy); !slices.Equal(got, test.wantLegacy) {
				t.Errorf("legacy = %q, want %q", got, test.wantLegacy)
			}
		})
	}
}

// For every literal and every single-family name, the legacy list is what
// net.ResolveIPAddr("ip", addr) gave.
func TestNgapListenAddrsLegacyMatchesResolveIPAddr(t *testing.T) {
	for _, addr := range []string{testWildcard4, testLoopback4, testN2IPv4, "::", testLoopback6, "2001:db8::18", "fe80::1%lo", "", "localhost"} {
		old, err := net.ResolveIPAddr("ip", addr)
		if err != nil {
			t.Fatalf("ResolveIPAddr(%q): %v", addr, err)
		}
		_, legacy := ngapListenAddrs([]string{addr})
		if len(legacy) != 1 || !legacy[0].IP.Equal(old.IP) || legacy[0].Zone != old.Zone {
			t.Errorf("legacy for %q = %v, want [%v]", addr, legacy, old)
		}
	}
}

func TestListenSCTPFallsBack(t *testing.T) {
	all := &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP(testN2IPv4)}, {IP: net.ParseIP(testLoopback6)}}}
	legacy := &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP(testN2IPv4)}}}
	sentinel := &sctp.SCTPListener{}
	errNoIPv6 := errors.New("address family not supported")

	var tried []*sctp.SCTPAddr
	listen := func(addr *sctp.SCTPAddr) (*sctp.SCTPListener, error) {
		tried = append(tried, addr)
		if addr == all {
			return nil, errNoIPv6
		}
		return sentinel, nil
	}
	if got, err := listenSCTP(listen, all, legacy); got != sentinel || err != nil {
		t.Fatalf("listenSCTP() = %v, %v; want the fallback listener", got, err)
	}
	if len(tried) != 2 || tried[0] != all || tried[1] != legacy {
		t.Fatalf("tried %v, want all then legacy", tried)
	}

	tried = nil
	if got, err := listenSCTP(listen, legacy); got != sentinel || err != nil || len(tried) != 1 {
		t.Fatalf("listenSCTP() with one candidate = %v, %v after %d tries", got, err, len(tried))
	}
	if _, err := listenSCTP(func(*sctp.SCTPAddr) (*sctp.SCTPListener, error) { return nil, errNoIPv6 }, all, legacy); !errors.Is(err, errNoIPv6) {
		t.Fatalf("listenSCTP() error = %v, want the last error", err)
	}
}

// A dual-stack address list really binds both families on one SCTP socket,
// and gNBs of either family can connect.
//
// It opens real sockets, so it must not run after a Run lifecycle in the same
// process: that lifecycle's accept loop cannot be stopped and keeps calling
// accept on its closed descriptor number, and would take over this test's
// listener when the kernel hands the number out again.
func TestDualStackNgapListenerAcceptsBothFamilies(t *testing.T) {
	if listenerLifecycleUsed {
		t.Skip("a Run lifecycle's accept loop is still running in this process")
	}
	addr := &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP(testLoopback4)}, {IP: net.ParseIP(testLoopback6)}}}
	listener, err := sctpConfig.Listen("sctp", addr)
	if err != nil {
		t.Skipf("cannot bind SCTP on 127.0.0.1 and ::1 here: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	bound, ok := listener.Addr().(*sctp.SCTPAddr)
	if !ok {
		t.Fatalf("listener address is %T", listener.Addr())
	}
	for _, ip := range []string{testLoopback4, testLoopback6} {
		raddr := &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP(ip)}}, Port: bound.Port}
		conn, err := sctp.DialSCTP("sctp", nil, raddr)
		if err != nil {
			t.Fatalf("dial %s: %v (listener on %s)", raddr, err, bound)
		}
		if err := conn.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}
