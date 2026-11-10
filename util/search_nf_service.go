// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package util

import (
	"github.com/omec-project/amf/context"
	"github.com/omec-project/openapi/v2/models"
	openapiutils "github.com/omec-project/openapi/v2/utils"
	"github.com/omec-project/util/sbiaddr"
)

// SearchNFServiceUri returns the base URI of the first service of a
// discovered NF with the given name and status, or "" when there is none.
// The shared resolver brackets IPv6 hosts and reads both address families,
// preferring the AMF's configured family (sbi.preferredFamily).
func SearchNFServiceUri(nfProfile models.NFProfileDiscovery, serviceName models.ServiceName,
	nfServiceStatus models.NFServiceStatus,
) (nfUri string) {
	return openapiutils.SearchNFServiceURI(&nfProfile, serviceName, nfServiceStatus,
		context.AMF_Self().SbiPreference == sbiaddr.PreferIPv6)
}
