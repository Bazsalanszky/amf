// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"testing"

	"github.com/omec-project/amf/context"
	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/util/sbiaddr"
)

func TestSearchNFServiceUriDualStack(t *testing.T) {
	const (
		peerIPv4    = "10.42.0.9"
		peerIPv6    = "2001:db8:600::9"
		peerIPv4URI = "http://" + peerIPv4 + ":29504"
		peerIPv6URI = "http://[" + peerIPv6 + "]:29504"
	)
	endPoint := func(ipv4, ipv6 string) models.IpEndPoint {
		e := models.IpEndPoint{}
		if ipv4 != "" {
			e.SetIpv4Address(ipv4)
		}
		if ipv6 != "" {
			e.SetIpv6Address(ipv6)
		}
		e.SetPort(29504)
		return e
	}
	profile := func(endPoints ...models.IpEndPoint) models.NFProfileDiscovery {
		service := models.NFService{}
		service.SetServiceName(models.SERVICENAME_NUDR_DR)
		service.SetNfServiceStatus(models.NFSERVICESTATUS_REGISTERED)
		service.SetScheme(models.URISCHEME_HTTP)
		service.SetIpEndPoints(endPoints)
		p := models.NFProfileDiscovery{}
		p.SetNfServices([]models.NFService{service})
		return p
	}
	tests := []struct {
		name    string
		profile models.NFProfileDiscovery
		pref    sbiaddr.Preference
		want    string
	}{
		{name: "IPv4 endpoint, as before", profile: profile(endPoint(peerIPv4, "")), want: peerIPv4URI},
		{name: "IPv6-only peer", profile: profile(endPoint("", peerIPv6)), want: peerIPv6URI},
		{
			name:    "dual-stack peer, IPv4 preferred",
			profile: profile(endPoint(peerIPv4, ""), endPoint("", peerIPv6)),
			want:    peerIPv4URI,
		},
		{
			name:    "dual-stack peer, IPv6 preferred",
			profile: profile(endPoint(peerIPv4, ""), endPoint("", peerIPv6)),
			pref:    sbiaddr.PreferIPv6,
			want:    peerIPv6URI,
		},
		{
			name:    "IPv6 preferred, IPv4-only peer",
			profile: profile(endPoint(peerIPv4, "")),
			pref:    sbiaddr.PreferIPv6,
			want:    peerIPv4URI,
		},
		{
			// Legacy peers put whatever they registered into ipv4Address.
			name:    "IPv6 address in ipv4Address",
			profile: profile(endPoint(peerIPv6, "")),
			want:    peerIPv6URI,
		},
	}
	self := context.AMF_Self()
	original := self.SbiPreference
	t.Cleanup(func() { self.SbiPreference = original })
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			self.SbiPreference = test.pref
			got := SearchNFServiceUri(test.profile, models.SERVICENAME_NUDR_DR, models.NFSERVICESTATUS_REGISTERED)
			if got != test.want {
				t.Fatalf("SearchNFServiceUri() = %q, want %q", got, test.want)
			}
		})
	}
}
