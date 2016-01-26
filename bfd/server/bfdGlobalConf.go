package server

import (
	//"fmt"
	"l3/bfd/config"
)

type GlobalConf struct {
	/*
	   RouterId                    []byte
	   AdminStat                   config.Status
	   ASBdrRtrStatus              bool
	   TOSSupport                  bool
	   ExtLsdbLimit                int32
	   MulticastExtensions         int32
	   ExitOverflowInterval        config.PositiveInteger
	   DemandExtensions            bool
	   RFC1583Compatibility        bool
	   ReferenceBandwidth          int32
	   RestartSupport              config.RestartSupport
	   RestartInterval             int32
	   RestartStrictLsaChecking    bool
	   StubRouterAdvertisement     config.AdvertiseAction
	   Version                     uint8
	*/
}

func (server *BFDServer) updateGlobalConf(gConf config.GlobalConf) {
	/*
	   routerId := convertAreaOrRouterId(string(gConf.RouterId))
	   if routerId == nil {
	       server.logger.Err("Invalid Router Id")
	       return
	   }
	   server.ospfGlobalConf.RouterId = routerId
	   server.ospfGlobalConf.AdminStat = gConf.AdminStat
	   server.ospfGlobalConf.ASBdrRtrStatus = gConf.ASBdrRtrStatus
	   server.ospfGlobalConf.TOSSupport = gConf.TOSSupport
	   server.ospfGlobalConf.ExtLsdbLimit = gConf.ExtLsdbLimit
	   server.ospfGlobalConf.MulticastExtensions = gConf.MulticastExtensions
	   server.ospfGlobalConf.ExitOverflowInterval = gConf.ExitOverflowInterval
	   server.ospfGlobalConf.RFC1583Compatibility = gConf.RFC1583Compatibility
	   server.ospfGlobalConf.ReferenceBandwidth = gConf.ReferenceBandwidth
	   server.ospfGlobalConf.RestartSupport = gConf.RestartSupport
	   server.ospfGlobalConf.RestartInterval= gConf.RestartInterval
	   server.ospfGlobalConf.RestartStrictLsaChecking = gConf.RestartStrictLsaChecking
	   server.ospfGlobalConf.StubRouterAdvertisement = gConf.StubRouterAdvertisement
	   server.logger.Err("Global configuration updated")
	*/
}

func (server *BFDServer) initBfdGlobalConfDefault() {
	/*
	   routerId := convertAreaOrRouterId("0.0.0.0")
	   if routerId == nil {
	       server.logger.Err("Invalid Router Id")
	       return
	   }
	   server.ospfGlobalConf.RouterId = routerId
	   server.ospfGlobalConf.AdminStat = config.Disabled
	   server.ospfGlobalConf.ASBdrRtrStatus = false
	   server.ospfGlobalConf.TOSSupport = false
	   server.ospfGlobalConf.ExtLsdbLimit = -1
	   server.ospfGlobalConf.MulticastExtensions = 0
	   server.ospfGlobalConf.ExitOverflowInterval = 0
	   server.ospfGlobalConf.RFC1583Compatibility = false
	   server.ospfGlobalConf.ReferenceBandwidth = 100000 // Default value 100 MBPS
	   server.ospfGlobalConf.RestartSupport = config.None
	   server.ospfGlobalConf.RestartInterval= 0
	   server.ospfGlobalConf.RestartStrictLsaChecking = false
	   server.ospfGlobalConf.StubRouterAdvertisement = config.DoNotAdvertise
	   server.ospfGlobalConf.Version = uint8(OSPF_VERSION_2)
	   server.logger.Err("Global configuration initialized")
	*/
}

func (server *BFDServer) processGlobalConfig(gConf config.GlobalConf) {
	/*
	   var localIntfStateMap = make(map[IntfConfKey]config.Status)
	   for key, ent := range server.IntfConfMap {
	       localIntfStateMap[key] = ent.IfAdminStat
	       if ent.IfAdminStat == config.Enabled &&
	           server.ospfGlobalConf.AdminStat == config.Enabled {
	           server.StopSendRecvPkts(key)
	       }
	   }

	   server.logger.Info(fmt.Sprintln("Received call for performing Global Configuration", gConf))
	   server.updateGlobalConf(gConf)

	   for key, ent := range localIntfStateMap {
	       if ent == config.Enabled &&
	           server.ospfGlobalConf.AdminStat == config.Enabled {
	           server.StartSendRecvPkts(key)
	       }
	   }
	*/
}