// SPDX-FileCopyrightText: 2024 Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package util

import (
	"cmp"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/omec-project/amf/context"
	"github.com/omec-project/amf/factory"
	"github.com/omec-project/amf/logger"
	"github.com/omec-project/nas/v2/security"
	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/util/drsm"
	"github.com/omec-project/util/sbiaddr"
)

// defaultDrsmMongoURL is the fallback MongoDB URL used when no MongoDB URL is
// configured. DRSM relies on MongoDB change streams (collection.Watch), which
// are only available on a replica set, so the default assumes a replica set
// named "rs0". Deployments using a different replica set name (or a managed
// MongoDB endpoint) must override this via the MongoDB URL configuration.
const defaultDrsmMongoURL = "mongodb://mongodb-headless:27017/?replicaSet=rs0"

const defaultNgapLocalIP = "127.0.0.1"

func resolveStableAmfNfId(configuration *factory.Configuration) string {
	if nfID := os.Getenv("NF_ID"); nfID != "" {
		if _, err := uuid.Parse(nfID); err == nil {
			return nfID
		}
		return uuid.NewSHA1(uuid.NameSpaceOID, []byte(nfID)).String()
	}
	if configuration != nil {
		if configuration.AmfName != "" {
			return uuid.NewSHA1(uuid.NameSpaceOID, []byte(configuration.AmfName)).String()
		}
		if configuration.Sbi != nil {
			// The FQDN, else the IPv4 address, else the IPv6 address: the same
			// string as before for IPv4 and FQDN registrations.
			if host := resolveRegisterAddrs(configuration.Sbi).Host(sbiaddr.PreferIPv4); host != "" {
				return uuid.NewSHA1(uuid.NameSpaceOID, []byte(host)).String()
			}
		}
	}
	return uuid.New().String()
}

// resolveRegisterAddrs resolves sbi.registerIPv4 and sbi.registerIPv6 into the
// addresses the AMF registers at the NRF. Either key may hold an IPv4 or IPv6
// address, a host name, or the name of an environment variable (POD_IP,
// POD_IPS); each address goes to the slot of its own family. Unusable values
// are logged and skipped; the result is empty when nothing usable is left.
func resolveRegisterAddrs(sbi *factory.Sbi) sbiaddr.RegisterAddrs {
	if sbi == nil {
		return sbiaddr.RegisterAddrs{}
	}
	addrs, err := sbiaddr.ResolveRegister(sbi.RegisterIPv4, sbi.RegisterIPv6)
	if err != nil {
		logger.UtilLog.Warnf("SBI register address: %v", err)
	}
	if addrs.Empty() && (sbi.RegisterIPv4 != "" || sbi.RegisterIPv6 != "") {
		logger.UtilLog.Warnf("no usable SBI register address; using default %s", factory.AMF_DEFAULT_IPV4)
	}
	if addrs.FQDN != "" && addrs.IPv4 != "" {
		logger.UtilLog.Warnf("SBI register address: registering FQDN %s, not IPv4 %s", addrs.FQDN, addrs.IPv4)
	}
	return addrs
}

func resolveDrsmMongoURL(configuration *factory.Configuration) string {
	if configuration != nil && configuration.Mongodb != nil && configuration.Mongodb.Url != "" {
		return configuration.Mongodb.Url
	}
	return defaultDrsmMongoURL
}

func InitDrsm() (drsm.DrsmInterface, error) {
	podname := os.Getenv("HOSTNAME")
	podip := os.Getenv("POD_IP")
	logger.UtilLog.Infof("NfId Instance: %v", context.AMF_Self().NfId)
	podId := drsm.PodId{PodName: podname, PodInstance: context.AMF_Self().NfId, PodIp: podip}
	logger.UtilLog.Debugf("PodId: %v", podId)
	dbUrl := resolveDrsmMongoURL(factory.AmfConfig.Configuration)
	opt := &drsm.Options{ResIdSize: 24, Mode: drsm.ResourceClient}
	db := drsm.DbInfo{Url: dbUrl, Name: factory.AmfConfig.Configuration.AmfDBName}

	// amfid is being used for amfngapid, subscriberid and tmsi for this release
	return drsm.InitDRSM("amfid", podId, db, opt)
}

func InitAmfContext(amfContext *context.AMFContext) {
	config := factory.AmfConfig
	logger.UtilLog.Infof("amfconfig Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)
	configuration := config.Configuration
	if amfContext.NfId == "" {
		amfContext.NfId = resolveStableAmfNfId(configuration)
	}

	if configuration.AmfName != "" {
		amfContext.Name = configuration.AmfName
	}
	amfContext.NgapIpList = []string{defaultNgapLocalIP} // default localhost
	if configuration.NgapIpList != nil {
		amfContext.NgapIpList = configuration.NgapIpList
	}
	amfContext.NgapPort = configuration.NgapPort
	amfContext.SctpGrpcPort = configuration.SctpGrpcPort
	sbi := configuration.Sbi
	if sbi.Scheme != "" {
		amfContext.UriScheme = models.UriScheme(sbi.Scheme)
	} else {
		logger.UtilLog.Warnln("SBI scheme has not been set. Using http as default")
		amfContext.UriScheme = "http"
	}
	addrs := resolveRegisterAddrs(sbi)
	if addrs.Empty() {
		addrs.IPv4 = factory.AMF_DEFAULT_IPV4 // default localhost
	}
	// An FQDN takes the legacy field, as before; IPv6 is registered next to it.
	amfContext.RegisterIPv4 = cmp.Or(addrs.FQDN, addrs.IPv4)
	amfContext.RegisterIPv6 = addrs.IPv6
	amfContext.SbiPreference = sbiaddr.PreferIPv4
	amfContext.SBIPort = factory.AMF_DEFAULT_PORT_INT // default port
	if sbi != nil {
		var err error
		if amfContext.SbiPreference, err = sbiaddr.ParsePreference(sbi.PreferredFamily); err != nil {
			logger.UtilLog.Warnf("SBI preferredFamily: %v; using IPv4", err)
		}
		if sbi.Port != 0 {
			amfContext.SBIPort = sbi.Port
		}
		if tls := sbi.TLS; tls != nil {
			if tls.Key != "" {
				amfContext.Key = tls.Key
			}
			if tls.PEM != "" {
				amfContext.PEM = tls.PEM
			}
		}
		// bindingAddr wins over its legacy alias bindingIPv4; either may name
		// an environment variable. The log lines are the ones operators knew.
		bindingAddr := cmp.Or(sbi.BindingAddr, sbi.BindingIPv4)
		if bindingAddr != "" && os.Getenv(bindingAddr) != "" {
			logger.UtilLog.Infoln("parsing ServerIPv4 address from ENV Variable")
		} else if bindingAddr == "" {
			logger.UtilLog.Warnln("error parsing ServerIPv4 address from string. Using the 0.0.0.0 as default")
		}
		if amfContext.BindingIPv4, err = sbiaddr.BindHost(sbi.BindingAddr, sbi.BindingIPv4); err != nil {
			logger.UtilLog.Warnf("SBI binding address: %v", err)
		}
	}
	serviceNameList := configuration.ServiceNameList
	amfContext.InitNFService(serviceNameList, config.Info.Version)
	amfContext.ServedGuamiList = []models.Guami{}
	amfContext.SupportTaiLists = []models.Tai{}
	amfContext.PlmnSupportList = []models.PlmnSnssai{}
	amfContext.SupportDnnLists = configuration.SupportDnnList
	if configuration.NrfUri != "" {
		amfContext.NrfUri = configuration.NrfUri
	} else {
		logger.UtilLog.Warnln("NRF Uri is empty! Using localhost as NRF IPv4 address")
		amfContext.NrfUri = factory.AMF_DEFAULT_NRFURI
	}
	security := configuration.Security
	if security != nil {
		amfContext.SecurityAlgorithm.IntegrityOrder = getIntAlgOrder(security.IntegrityOrder)
		amfContext.SecurityAlgorithm.CipheringOrder = getEncAlgOrder(security.CipheringOrder)
	}
	amfContext.NetworkName = configuration.NetworkName
	amfContext.T3502Value = configuration.T3502Value
	amfContext.T3512Value = configuration.T3512Value
	amfContext.Non3gppDeregistrationTimerValue = configuration.Non3gppDeregistrationTimerValue
	amfContext.T3513Cfg = configuration.T3513
	amfContext.T3522Cfg = configuration.T3522
	amfContext.T3550Cfg = configuration.T3550
	amfContext.T3560Cfg = configuration.T3560
	amfContext.T3565Cfg = configuration.T3565
	amfContext.EnableSctpLb = configuration.EnableSctpLb
	amfContext.EnableDbStore = configuration.EnableDbStore
	amfContext.EnableNrfCaching = configuration.EnableNrfCaching
	if configuration.EnableNrfCaching {
		if configuration.NrfCacheEvictionInterval == 0 {
			amfContext.NrfCacheEvictionInterval = time.Duration(900) // 15 mins
		} else {
			amfContext.NrfCacheEvictionInterval = time.Duration(configuration.NrfCacheEvictionInterval)
		}
	}
}

func getIntAlgOrder(integrityOrder []string) (intOrder []uint8) {
	for _, intAlg := range integrityOrder {
		switch intAlg {
		case "NIA0":
			intOrder = append(intOrder, security.AlgIntegrity128NIA0)
		case "NIA1":
			intOrder = append(intOrder, security.AlgIntegrity128NIA1)
		case "NIA2":
			intOrder = append(intOrder, security.AlgIntegrity128NIA2)
		case "NIA3":
			intOrder = append(intOrder, security.AlgIntegrity128NIA3)
		default:
			logger.UtilLog.Errorf("unsupported algorithm: %s", intAlg)
		}
	}
	return
}

func getEncAlgOrder(cipheringOrder []string) (encOrder []uint8) {
	for _, encAlg := range cipheringOrder {
		switch encAlg {
		case "NEA0":
			encOrder = append(encOrder, security.AlgCiphering128NEA0)
		case "NEA1":
			encOrder = append(encOrder, security.AlgCiphering128NEA1)
		case "NEA2":
			encOrder = append(encOrder, security.AlgCiphering128NEA2)
		case "NEA3":
			encOrder = append(encOrder, security.AlgCiphering128NEA3)
		default:
			logger.UtilLog.Errorf("unsupported algorithm: %s", encAlg)
		}
	}
	return
}
