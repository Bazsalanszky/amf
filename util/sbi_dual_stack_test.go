// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"testing"

	"github.com/google/uuid"
	"github.com/omec-project/amf/context"
	"github.com/omec-project/amf/factory"
	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/util/sbiaddr"
)

const (
	testPodIPv4    = "10.42.0.5"
	testPodIPv6    = "2001:db8:600::5"
	testSbiPort    = 29518
	testFqdnURI    = "http://" + testAMFHostname + ":29518"
	testIPv4URI    = "http://" + testPodIPv4 + ":29518"
	testIPv6URI    = "http://[" + testPodIPv6 + "]:29518"
	testDefaultURI = "http://" + factory.AMF_DEFAULT_IPV4 + ":29518"
)

// initSbiContext runs InitAmfContext on a fresh context with only the given
// SBI section configured.
func initSbiContext(t *testing.T, sbi *factory.Sbi) *context.AMFContext {
	t.Helper()
	original := factory.AmfConfig
	t.Cleanup(func() { factory.AmfConfig = original })
	factory.AmfConfig = factory.Config{
		Info: &factory.Info{Version: "1.0.0"},
		Configuration: &factory.Configuration{
			AmfName:         testAMFHostname,
			Sbi:             sbi,
			ServiceNameList: []string{string(models.SERVICENAME_NAMF_COMM)},
		},
	}
	amfCtx := &context.AMFContext{NfService: make(map[models.ServiceName]models.NFService)}
	InitAmfContext(amfCtx)
	return amfCtx
}

func TestInitAmfContextSbiAddresses(t *testing.T) {
	tests := []struct {
		name         string
		sbi          factory.Sbi
		env          map[string]string
		wantIPv4     string // context RegisterIPv4: IPv4 address or FQDN
		wantIPv6     string
		wantPref     sbiaddr.Preference
		wantBind     string
		wantURI      string
		wantEndPoint []string // ipv4Address or ipv6Address of each IpEndPoint
	}{
		{
			name:         "chart default: service name",
			sbi:          factory.Sbi{RegisterIPv4: testAMFHostname, BindingIPv4: sbiaddr.DefaultBindHost},
			wantIPv4:     testAMFHostname,
			wantBind:     sbiaddr.DefaultBindHost,
			wantURI:      testFqdnURI,
			wantEndPoint: []string{""},
		},
		{
			name:         "IPv4 literal",
			sbi:          factory.Sbi{RegisterIPv4: testPodIPv4},
			wantIPv4:     testPodIPv4,
			wantBind:     sbiaddr.DefaultBindHost,
			wantURI:      testIPv4URI,
			wantEndPoint: []string{testPodIPv4},
		},
		{
			name:         "POD_IP holding IPv4",
			sbi:          factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIP, BindingIPv4: sbiaddr.EnvPodIP},
			env:          map[string]string{sbiaddr.EnvPodIP: testPodIPv4},
			wantIPv4:     testPodIPv4,
			wantBind:     testPodIPv4,
			wantURI:      testIPv4URI,
			wantEndPoint: []string{testPodIPv4},
		},
		{
			// Before: rejected by the To4() check, and 127.0.0.18 was registered.
			name:         "POD_IP holding IPv6 under registerIPv4",
			sbi:          factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIP, BindingIPv4: sbiaddr.EnvPodIP},
			env:          map[string]string{sbiaddr.EnvPodIP: testPodIPv6},
			wantIPv6:     testPodIPv6,
			wantBind:     testPodIPv6,
			wantURI:      testIPv6URI,
			wantEndPoint: []string{testPodIPv6},
		},
		{
			name:         "POD_IPS dual stack",
			sbi:          factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIPs},
			env:          map[string]string{sbiaddr.EnvPodIPs: testPodIPv4 + "," + testPodIPv6},
			wantIPv4:     testPodIPv4,
			wantIPv6:     testPodIPv6,
			wantBind:     sbiaddr.DefaultBindHost,
			wantURI:      testIPv4URI,
			wantEndPoint: []string{testPodIPv4, testPodIPv6},
		},
		{
			name:         "POD_IPS dual stack, IPv6 primary, preferredFamily IPv6",
			sbi:          factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIPs, PreferredFamily: "IPv6"},
			env:          map[string]string{sbiaddr.EnvPodIPs: testPodIPv6 + "," + testPodIPv4},
			wantIPv4:     testPodIPv4,
			wantIPv6:     testPodIPv6,
			wantPref:     sbiaddr.PreferIPv6,
			wantBind:     sbiaddr.DefaultBindHost,
			wantURI:      testIPv6URI,
			wantEndPoint: []string{testPodIPv4, testPodIPv6},
		},
		{
			name:         "registerIPv4 and registerIPv6",
			sbi:          factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIP, RegisterIPv6: "2001:DB8:600:0::5"},
			env:          map[string]string{sbiaddr.EnvPodIP: testPodIPv4},
			wantIPv4:     testPodIPv4,
			wantIPv6:     testPodIPv6,
			wantBind:     sbiaddr.DefaultBindHost,
			wantURI:      testIPv4URI,
			wantEndPoint: []string{testPodIPv4, testPodIPv6},
		},
		{
			name:         "FQDN plus IPv6",
			sbi:          factory.Sbi{RegisterIPv4: testAMFHostname, RegisterIPv6: sbiaddr.EnvPodIP, PreferredFamily: "IPv6"},
			env:          map[string]string{sbiaddr.EnvPodIP: testPodIPv6},
			wantIPv4:     testAMFHostname,
			wantIPv6:     testPodIPv6,
			wantPref:     sbiaddr.PreferIPv6,
			wantBind:     sbiaddr.DefaultBindHost,
			wantURI:      testFqdnURI,
			wantEndPoint: []string{testPodIPv6},
		},
		{
			name:         "unset POD_IP falls back to the default",
			sbi:          factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIP},
			env:          map[string]string{sbiaddr.EnvPodIP: ""},
			wantIPv4:     factory.AMF_DEFAULT_IPV4,
			wantBind:     sbiaddr.DefaultBindHost,
			wantURI:      testDefaultURI,
			wantEndPoint: []string{factory.AMF_DEFAULT_IPV4},
		},
		{
			name:         "nothing configured",
			sbi:          factory.Sbi{},
			wantIPv4:     factory.AMF_DEFAULT_IPV4,
			wantBind:     sbiaddr.DefaultBindHost,
			wantURI:      testDefaultURI,
			wantEndPoint: []string{factory.AMF_DEFAULT_IPV4},
		},
		{
			name:         "bindingAddr wins over bindingIPv4",
			sbi:          factory.Sbi{RegisterIPv4: testPodIPv4, BindingAddr: "::", BindingIPv4: sbiaddr.DefaultBindHost},
			wantIPv4:     testPodIPv4,
			wantBind:     "::",
			wantURI:      testIPv4URI,
			wantEndPoint: []string{testPodIPv4},
		},
		{
			name:         "bindingAddr from POD_IP",
			sbi:          factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIP, BindingAddr: sbiaddr.EnvPodIP},
			env:          map[string]string{sbiaddr.EnvPodIP: testPodIPv6},
			wantIPv6:     testPodIPv6,
			wantBind:     testPodIPv6,
			wantURI:      testIPv6URI,
			wantEndPoint: []string{testPodIPv6},
		},
		{
			name:         "bracketed bindingIPv4 is unwrapped",
			sbi:          factory.Sbi{RegisterIPv4: testPodIPv4, BindingIPv4: "[::]"},
			wantIPv4:     testPodIPv4,
			wantBind:     "::",
			wantURI:      testIPv4URI,
			wantEndPoint: []string{testPodIPv4},
		},
		{
			name:         "unknown preferredFamily is IPv4",
			sbi:          factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIPs, PreferredFamily: "IPv5"},
			env:          map[string]string{sbiaddr.EnvPodIPs: testPodIPv4 + "," + testPodIPv6},
			wantIPv4:     testPodIPv4,
			wantIPv6:     testPodIPv6,
			wantBind:     sbiaddr.DefaultBindHost,
			wantURI:      testIPv4URI,
			wantEndPoint: []string{testPodIPv4, testPodIPv6},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(sbiaddr.EnvPodIP, "")
			t.Setenv(sbiaddr.EnvPodIPs, "")
			for k, v := range test.env {
				t.Setenv(k, v)
			}
			sbi := test.sbi
			sbi.Scheme = string(models.URISCHEME_HTTP)
			sbi.Port = testSbiPort
			amfCtx := initSbiContext(t, &sbi)

			if amfCtx.RegisterIPv4 != test.wantIPv4 {
				t.Errorf("RegisterIPv4 = %q, want %q", amfCtx.RegisterIPv4, test.wantIPv4)
			}
			if amfCtx.RegisterIPv6 != test.wantIPv6 {
				t.Errorf("RegisterIPv6 = %q, want %q", amfCtx.RegisterIPv6, test.wantIPv6)
			}
			if amfCtx.SbiPreference != test.wantPref {
				t.Errorf("SbiPreference = %v, want %v", amfCtx.SbiPreference, test.wantPref)
			}
			if amfCtx.BindingIPv4 != test.wantBind {
				t.Errorf("BindingIPv4 = %q, want %q", amfCtx.BindingIPv4, test.wantBind)
			}
			if got := amfCtx.GetSbiUri(); got != test.wantURI {
				t.Errorf("GetSbiUri() = %q, want %q", got, test.wantURI)
			}
			service := amfCtx.NfService[models.SERVICENAME_NAMF_COMM]
			if got := service.GetApiPrefix(); got != test.wantURI {
				t.Errorf("apiPrefix = %q, want %q", got, test.wantURI)
			}
			var endPoints []string
			for _, endPoint := range service.GetIpEndPoints() {
				endPoints = append(endPoints, endPoint.GetIpv4Address()+endPoint.GetIpv6Address())
				if endPoint.GetPort() != testSbiPort {
					t.Errorf("endpoint port = %d, want %d", endPoint.GetPort(), testSbiPort)
				}
			}
			if len(endPoints) != len(test.wantEndPoint) {
				t.Fatalf("endpoints = %q, want %q", endPoints, test.wantEndPoint)
			}
			for i := range endPoints {
				if endPoints[i] != test.wantEndPoint[i] {
					t.Errorf("endpoints = %q, want %q", endPoints, test.wantEndPoint)
				}
			}
		})
	}
}

func TestResolveStableAmfNfIdDualStack(t *testing.T) {
	hash := func(s string) string { return uuid.NewSHA1(uuid.NameSpaceOID, []byte(s)).String() }
	tests := []struct {
		name string
		sbi  factory.Sbi
		env  map[string]string
		want string
	}{
		{name: "IPv4 literal, as before", sbi: factory.Sbi{RegisterIPv4: testPodIPv4}, want: hash(testPodIPv4)},
		{name: "FQDN, as before", sbi: factory.Sbi{RegisterIPv4: testAMFHostname}, want: hash(testAMFHostname)},
		{
			name: "POD_IP holding IPv4, as before",
			sbi:  factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIP},
			env:  map[string]string{sbiaddr.EnvPodIP: testPodIPv4},
			want: hash(testPodIPv4),
		},
		{
			name: "FQDN wins over an IPv6 registerIPv6",
			sbi:  factory.Sbi{RegisterIPv4: testAMFHostname, RegisterIPv6: testPodIPv6},
			want: hash(testAMFHostname),
		},
		{
			name: "IPv4 wins in dual stack",
			sbi:  factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIPs},
			env:  map[string]string{sbiaddr.EnvPodIPs: testPodIPv6 + "," + testPodIPv4},
			want: hash(testPodIPv4),
		},
		{
			// Before: rejected, so the ID was a random UUID.
			name: "POD_IP holding IPv6",
			sbi:  factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIP},
			env:  map[string]string{sbiaddr.EnvPodIP: testPodIPv6},
			want: hash(testPodIPv6),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("NF_ID", "")
			for k, v := range test.env {
				t.Setenv(k, v)
			}
			sbi := test.sbi
			if got := resolveStableAmfNfId(&factory.Configuration{Sbi: &sbi}); got != test.want {
				t.Fatalf("resolveStableAmfNfId() = %q, want %q", got, test.want)
			}
		})
	}
}
