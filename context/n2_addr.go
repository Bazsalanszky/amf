// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"errors"
	"net/netip"
	"strings"
)

// N2Addrs returns the gNB's N2 transport addresses, one per family, taken
// from GnbIp. Either is "" when the association has no usable address of
// that family.
func (ran *AmfRan) N2Addrs() (ipv4, ipv6 string) {
	return parseN2Addrs(ran.GnbIp)
}

// parseN2Addrs reads a remote SCTP address in the form the AMF records it:
// net.Conn.RemoteAddr().String() of the direct listener, or the same string
// forwarded by sctplb. For an *sctp.SCTPAddr that is every address of the
// association joined with "/", IPv6 in brackets, then ":port", for example
// "10.0.0.1:38412", "[2001:db8::1]:38412" or "[2001:db8::1]/10.0.0.1:38412".
// A bare address without a port is accepted too.
//
// It returns the first IPv4 and the first IPv6 address. IPv4-mapped IPv6 counts
// as IPv4. An IPv6 address with a zone is skipped: the zone only means
// something on this host, and TS 29.571 Ipv6Addr has no zone.
func parseN2Addrs(remote string) (ipv4, ipv6 string) {
	for _, host := range n2Hosts(remote) {
		addr, err := parseN2Host(host)
		if err != nil {
			continue
		}
		addr = addr.Unmap()
		switch {
		case addr.Is4():
			if ipv4 == "" {
				ipv4 = addr.String()
			}
		case addr.Zone() != "":
		case ipv6 == "":
			ipv6 = addr.String()
		}
	}
	return ipv4, ipv6
}

// n2Hosts splits a remote SCTP address into its hosts, dropping the port.
func n2Hosts(remote string) []string {
	remote = strings.TrimSpace(remote)
	if _, err := parseN2Host(remote); err == nil {
		return []string{remote}
	}
	// The port follows the last colon outside brackets.
	depth, colon := 0, -1
	for i, c := range remote {
		switch c {
		case '[':
			depth++
		case ']':
			depth--
		case ':':
			if depth == 0 {
				colon = i
			}
		}
	}
	if colon >= 0 {
		remote = remote[:colon]
	}
	return strings.Split(remote, "/")
}

// parseN2Host parses one host: an address, or an IPv6 address in brackets.
// netip.ParseAddr takes everything after a "%" as the zone, so a string that
// still holds a list separator or a stray bracket is refused first.
func parseN2Host(host string) (netip.Addr, error) {
	if inner, ok := strings.CutPrefix(host, "["); ok {
		if inner, ok = strings.CutSuffix(inner, "]"); ok {
			host = inner
		}
	}
	if strings.ContainsAny(host, "[]/") {
		return netip.Addr{}, errInvalidN2Host
	}
	return netip.ParseAddr(host)
}

var errInvalidN2Host = errors.New("not a single N2 host address")
