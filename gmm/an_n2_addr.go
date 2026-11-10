// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package gmm

import (
	"github.com/omec-project/amf/context"
	"github.com/omec-project/openapi/v2/models"
)

// setAnN2Addrs puts the gNB's N2 address into the registration context sent
// to the target AMF, in the attribute of its family (TS 29.518
// RegistrationContextContainer anN2IPv4Addr / anN2IPv6Addr). A dual-stack
// multi-homed association fills both. GnbIp holds "addr:port", and used to be
// copied into anN2IPv4Addr whole, which matches neither Ipv4Addr nor Ipv6Addr.
func setAnN2Addrs(registerContext *models.RegistrationContextContainer, ran *context.AmfRan) {
	if registerContext == nil || ran == nil {
		return
	}
	ipv4, ipv6 := ran.N2Addrs()
	if ipv4 != "" {
		registerContext.SetAnN2IPv4Addr(ipv4)
	}
	if ipv6 != "" {
		registerContext.SetAnN2IPv6Addr(ipv6)
	}
	if ipv4 == "" && ipv6 == "" {
		ran.Log.Warnf("no N2 address of the gNB in %q; anN2IPv4Addr and anN2IPv6Addr are left out", ran.GnbIp)
	}
}
