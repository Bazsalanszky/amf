// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package gmm

import (
	"testing"

	"github.com/omec-project/amf/context"
	"github.com/omec-project/amf/logger"
	"github.com/omec-project/openapi/v2"
	"github.com/omec-project/openapi/v2/models"
)

func TestSetAnN2Addrs(t *testing.T) {
	const (
		gnbIPv4 = "10.10.2.232"
		gnbIPv6 = "2001:db8:620::232"
	)
	tests := []struct {
		name     string
		gnbIP    string
		wantIPv4 string
		wantIPv6 string
	}{
		// Before: anN2IPv4Addr was "10.10.2.232:38412".
		{name: "IPv4 gNB", gnbIP: gnbIPv4 + ":38412", wantIPv4: gnbIPv4},
		// Before: anN2IPv4Addr was "[2001:db8:620::232]:38412".
		{name: "IPv6 gNB", gnbIP: "[" + gnbIPv6 + "]:38412", wantIPv6: gnbIPv6},
		{name: "dual-stack multi-homed gNB", gnbIP: "[" + gnbIPv6 + "]/" + gnbIPv4 + ":38412", wantIPv4: gnbIPv4, wantIPv6: gnbIPv6},
		{name: "no address", gnbIP: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ran := &context.AmfRan{GnbIp: test.gnbIP, Log: logger.GmmLog}
			registerContext := models.NewRegistrationContextContainerWithDefaults()
			registerContext.SetInitialAmfName("amf.example.org") // required, and has a pattern of its own
			setAnN2Addrs(registerContext, ran)
			if got := registerContext.GetAnN2IPv4Addr(); got != test.wantIPv4 {
				t.Errorf("anN2IPv4Addr = %q, want %q", got, test.wantIPv4)
			}
			if got := registerContext.GetAnN2IPv6Addr(); got != test.wantIPv6 {
				t.Errorf("anN2IPv6Addr = %q, want %q", got, test.wantIPv6)
			}
			// The model's own Ipv4Addr / Ipv6Addr patterns.
			if err := openapi.Validate(registerContext); err != nil {
				t.Errorf("registration context does not validate: %v", err)
			}
		})
	}
	setAnN2Addrs(nil, &context.AmfRan{})
	setAnN2Addrs(models.NewRegistrationContextContainerWithDefaults(), nil)
}
