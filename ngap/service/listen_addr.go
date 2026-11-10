// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	gocontext "context"
	"net"
	"slices"
	"strings"

	"github.com/ishidawataru/sctp"
	"github.com/omec-project/amf/logger"
)

// lookupIPAddr resolves a host name; tests replace it.
var lookupIPAddr = net.DefaultResolver.LookupIPAddr

// ngapListenAddrs turns the configured ngapIpList into the SCTP addresses to
// bind.
//
// An IP literal is used as it is (brackets around IPv6 are accepted). A host
// name contributes every address it resolves to. net.ResolveIPAddr, which was
// used before, keeps only one of them, the IPv4 one when there is one, so a
// name with A and AAAA records bound IPv4 only and IPv6 gNBs could not
// connect. That address still comes first for each name; duplicates are
// dropped.
//
// legacy is the address list the old code produced: one address per name,
// IPv4 first. Run falls back to it when binding all of them fails, for example
// because a name resolves to ::1 in a pod that has IPv6 disabled.
func ngapListenAddrs(addresses []string) (all, legacy []net.IPAddr) {
	for _, addr := range addresses {
		host := strings.TrimSpace(addr)
		if inner, ok := strings.CutPrefix(host, "["); ok {
			host = strings.TrimSuffix(inner, "]")
		}
		if host == "" {
			// net.ResolveIPAddr gave an empty address, which binds the wildcard.
			all = appendIPAddr(all, net.IPAddr{})
			legacy = appendIPAddr(legacy, net.IPAddr{})
			continue
		}
		if ip := parseIPZone(host); ip != nil {
			all = appendIPAddr(all, *ip)
			legacy = appendIPAddr(legacy, *ip)
			logger.NgapLog.Debugf("NGAP address '%s' is %s", addr, ip)
			continue
		}
		resolved, err := lookupIPAddr(gocontext.Background(), host)
		if err != nil || len(resolved) == 0 {
			logger.NgapLog.Errorf("error resolving address '%s': %v", addr, err)
			continue
		}
		first := firstIPv4(resolved)
		legacy = appendIPAddr(legacy, first)
		all = appendIPAddr(all, first)
		for _, ip := range resolved {
			all = appendIPAddr(all, ip)
		}
		logger.NgapLog.Debugf("resolved address '%s' to %v", addr, resolved)
	}
	return all, legacy
}

// parseIPZone parses an IP literal with an optional IPv6 zone.
func parseIPZone(host string) *net.IPAddr {
	ip, zone, _ := strings.Cut(host, "%")
	parsed := net.ParseIP(ip)
	if parsed == nil || (zone != "" && parsed.To4() != nil) {
		return nil
	}
	return &net.IPAddr{IP: parsed, Zone: zone}
}

// firstIPv4 is the address net.ResolveIPAddr("ip", name) picks: the first
// IPv4 one, else the first one.
func firstIPv4(addrs []net.IPAddr) net.IPAddr {
	for _, a := range addrs {
		if a.IP.To4() != nil {
			return a
		}
	}
	return addrs[0]
}

func appendIPAddr(list []net.IPAddr, addr net.IPAddr) []net.IPAddr {
	if slices.ContainsFunc(list, func(a net.IPAddr) bool { return a.IP.Equal(addr.IP) && a.Zone == addr.Zone }) {
		return list
	}
	return append(list, addr)
}

// listenSCTP binds the first address list that works. With a single candidate
// it behaves as a plain Listen.
func listenSCTP(listen func(*sctp.SCTPAddr) (*sctp.SCTPListener, error), candidates ...*sctp.SCTPAddr) (*sctp.SCTPListener, error) {
	var err error
	for i, addr := range candidates {
		var listener *sctp.SCTPListener
		if listener, err = listen(addr); err == nil {
			return listener, nil
		}
		if i < len(candidates)-1 {
			logger.NgapLog.Warnf("failed to listen on %s: %+v; retrying on %s", addr, err, candidates[i+1])
		}
	}
	return nil, err
}
