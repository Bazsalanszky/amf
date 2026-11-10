// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"slices"
	"testing"

	amf_context "github.com/omec-project/amf/context"
	"github.com/omec-project/amf/factory"
	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/util/sbiaddr"
)

func TestGetNfProfileAddressFamilies(t *testing.T) {
	const (
		ipv4    = "10.42.0.5"
		ipv6    = "2001:db8:600::5"
		ipv4URI = "http://" + ipv4 + ":29518"
		ipv6URI = "http://[" + ipv6 + "]:29518"
		fqdnURI = "http://" + nfName + ":29518"
	)
	tests := []struct {
		name         string
		registerIPv4 string
		registerIPv6 string
		pref         sbiaddr.Preference
		wantFqdn     string
		wantIPv4     []string
		wantIPv6     []string
		wantEndPoint []string // ipv4Address or ipv6Address of each IpEndPoint
		wantURI      string
	}{
		{
			name: "IPv4", registerIPv4: ipv4,
			wantIPv4: []string{ipv4}, wantEndPoint: []string{ipv4}, wantURI: ipv4URI,
		},
		{
			name: "FQDN", registerIPv4: nfName,
			wantFqdn: nfName, wantEndPoint: []string{""}, wantURI: fqdnURI,
		},
		{
			name: "IPv6 in RegisterIPv4", registerIPv4: ipv6,
			wantIPv6: []string{ipv6}, wantEndPoint: []string{ipv6}, wantURI: ipv6URI,
		},
		{
			name: "IPv6 only", registerIPv6: ipv6,
			wantIPv6: []string{ipv6}, wantEndPoint: []string{ipv6}, wantURI: ipv6URI,
		},
		{
			name: "dual stack", registerIPv4: ipv4, registerIPv6: ipv6,
			wantIPv4: []string{ipv4}, wantIPv6: []string{ipv6},
			wantEndPoint: []string{ipv4, ipv6}, wantURI: ipv4URI,
		},
		{
			name: "dual stack, IPv6 preferred", registerIPv4: ipv4, registerIPv6: ipv6, pref: sbiaddr.PreferIPv6,
			wantIPv4: []string{ipv4}, wantIPv6: []string{ipv6},
			wantEndPoint: []string{ipv4, ipv6}, wantURI: ipv6URI,
		},
		{
			name: "FQDN plus IPv6", registerIPv4: nfName, registerIPv6: ipv6, pref: sbiaddr.PreferIPv6,
			wantFqdn: nfName, wantIPv6: []string{ipv6}, wantEndPoint: []string{ipv6}, wantURI: fqdnURI,
		},
	}

	originalConfig := factory.AmfConfig
	factory.AmfConfig = factory.Config{Configuration: &factory.Configuration{AmfId: testAmfID}}
	t.Cleanup(func() { factory.AmfConfig = originalConfig })

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			amfCtx := &amf_context.AMFContext{
				NfId:          testNfInstanceID,
				RegisterIPv4:  test.registerIPv4,
				RegisterIPv6:  test.registerIPv6,
				SbiPreference: test.pref,
				UriScheme:     models.URISCHEME_HTTP,
				SBIPort:       29518,
				NfService:     make(map[models.ServiceName]models.NFService),
			}
			amfCtx.InitNFService([]string{string(models.SERVICENAME_NAMF_COMM)}, "1.0.0")

			profile, err := getNfProfile(amfCtx, makeAccessAndMobilityConfig())
			if err != nil {
				t.Fatalf("getNfProfile() error = %v", err)
			}
			if profile.GetFqdn() != test.wantFqdn {
				t.Errorf("profile fqdn = %q, want %q", profile.GetFqdn(), test.wantFqdn)
			}
			if !slices.Equal(profile.Ipv4Addresses, test.wantIPv4) {
				t.Errorf("profile ipv4Addresses = %q, want %q", profile.Ipv4Addresses, test.wantIPv4)
			}
			if !slices.Equal(profile.Ipv6Addresses, test.wantIPv6) {
				t.Errorf("profile ipv6Addresses = %q, want %q", profile.Ipv6Addresses, test.wantIPv6)
			}
			service := profile.GetNfServiceList()["0"]
			if service.GetFqdn() != test.wantFqdn {
				t.Errorf("service fqdn = %q, want %q", service.GetFqdn(), test.wantFqdn)
			}
			var endPoints []string
			for _, endPoint := range service.GetIpEndPoints() {
				if endPoint.Ipv4Address != nil && endPoint.Ipv6Address != nil {
					t.Errorf("endpoint %+v carries both families", endPoint)
				}
				endPoints = append(endPoints, endPoint.GetIpv4Address()+endPoint.GetIpv6Address())
			}
			if !slices.Equal(endPoints, test.wantEndPoint) {
				t.Errorf("endpoints = %q, want %q", endPoints, test.wantEndPoint)
			}
			if service.GetApiPrefix() != test.wantURI {
				t.Errorf("service apiPrefix = %q, want %q", service.GetApiPrefix(), test.wantURI)
			}
			wantCallback := test.wantURI + "/namf-callback/v1/n1-message-notify"
			if callback := profile.DefaultNotificationSubscriptions[0].CallbackUri; callback != wantCallback {
				t.Errorf("callbackUri = %q, want %q", callback, wantCallback)
			}
		})
	}
}

func TestGetNfProfileRejectsEmptyAddress(t *testing.T) {
	originalConfig := factory.AmfConfig
	factory.AmfConfig = factory.Config{Configuration: &factory.Configuration{AmfId: testAmfID}}
	t.Cleanup(func() { factory.AmfConfig = originalConfig })

	amfCtx := &amf_context.AMFContext{NfId: testNfInstanceID, NfService: make(map[models.ServiceName]models.NFService)}
	if _, err := getNfProfile(amfCtx, makeAccessAndMobilityConfig()); err == nil || err.Error() != "AMF Address is empty" {
		t.Fatalf("getNfProfile() error = %v, want %q", err, "AMF Address is empty")
	}
}
