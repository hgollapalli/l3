//
//Copyright [2016] [SnapRoute Inc]
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//	 Unless required by applicable law or agreed to in writing, software
//	 distributed under the License is distributed on an "AS IS" BASIS,
//	 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	 See the License for the specific language governing permissions and
//	 limitations under the License.
//
// _______  __       __________   ___      _______.____    __    ____  __  .___________.  ______  __    __  
// |   ____||  |     |   ____\  \ /  /     /       |\   \  /  \  /   / |  | |           | /      ||  |  |  | 
// |  |__   |  |     |  |__   \  V  /     |   (----` \   \/    \/   /  |  | `---|  |----`|  ,----'|  |__|  | 
// |   __|  |  |     |   __|   >   <       \   \      \            /   |  |     |  |     |  |     |   __   | 
// |  |     |  `----.|  |____ /  .  \  .----)   |      \    /\    /    |  |     |  |     |  `----.|  |  |  | 
// |__|     |_______||_______/__/ \__\ |_______/        \__/  \__/     |__|     |__|      \______||__|  |__| 
//                                                                                                           

package FSMgr

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"l3/bgp/api"
	"l3/bgp/config"
	"l3/bgp/rpc"
	"l3/rib/ribdCommonDefs"
	"ribd"
	"ribdInt"
	"utils/logging"

	nanomsg "github.com/op/go-nanomsg"
)

/*  Init route manager with ribd client as its core
 */
func NewFSRouteMgr(logger *logging.Writer, fileName string) (*FSRouteMgr, error) {
	var ribdClient *ribd.RIBDServicesClient = nil
	ribdClientChan := make(chan *ribd.RIBDServicesClient)

	logger.Info("Connecting to RIBd")
	go rpc.StartRibdClient(logger, fileName, ribdClientChan)
	ribdClient = <-ribdClientChan
	if ribdClient == nil {
		logger.Err("Failed to connect to RIBd\n")
		return nil, errors.New("Failed to connect to RIBd")
	} else {
		logger.Info("Connected to RIBd")
	}

	mgr := &FSRouteMgr{
		plugin:     "ovsdb",
		ribdClient: ribdClient,
		logger:     logger,
	}

	return mgr, nil
}

/*  Start nano msg socket with ribd
 */
func (mgr *FSRouteMgr) Start() {
	mgr.ribSubSocket, _ = mgr.setupSubSocket(ribdCommonDefs.PUB_SOCKET_ADDR)
	mgr.ribSubBGPSocket, _ = mgr.setupSubSocket(ribdCommonDefs.PUB_SOCKET_BGPD_ADDR)
	go mgr.listenForRIBUpdates(mgr.ribSubSocket)
	go mgr.listenForRIBUpdates(mgr.ribSubBGPSocket)
}

func (mgr *FSRouteMgr) setupSubSocket(address string) (*nanomsg.SubSocket, error) {
	var err error
	var socket *nanomsg.SubSocket
	if socket, err = nanomsg.NewSubSocket(); err != nil {
		mgr.logger.Err(fmt.Sprintf("Failed to create subscribe socket %s",
			"error:%s", address, err))
		return nil, err
	}

	if err = socket.Subscribe(""); err != nil {
		mgr.logger.Err(fmt.Sprintf("Failed to subscribe to \"\" on ",
			"subscribe socket %s, error:%s", address, err))
		return nil, err
	}

	if _, err = socket.Connect(address); err != nil {
		mgr.logger.Err(fmt.Sprintf("Failed to connect to publisher socket %s,",
			"error:%s", address, err))
		return nil, err
	}

	mgr.logger.Info(fmt.Sprintf("Connected to publisher socker %s", address))
	if err = socket.SetRecvBuffer(1024 * 1024); err != nil {
		mgr.logger.Err(fmt.Sprintln("Failed to set the buffer size for",
			"subsriber socket %s, error:", address, err))
		return nil, err
	}
	return socket, nil
}

func (mgr *FSRouteMgr) listenForRIBUpdates(socket *nanomsg.SubSocket) {
	for {
		mgr.logger.Info("Read on RIB subscriber socket...")
		rxBuf, err := socket.Recv(0)
		if err != nil {
			mgr.logger.Err(fmt.Sprintln("Recv on RIB subscriber socket",
				"failed with error:", err))
			continue
		}
		mgr.logger.Info(fmt.Sprintln("RIB subscriber recv returned:", rxBuf))
		mgr.handleRibUpdates(rxBuf)
	}
}

func (mgr *FSRouteMgr) populateConfigRoute(route *ribdInt.Routes) *config.RouteInfo {
	rv := &config.RouteInfo{
		IPAddr:           route.Ipaddr,
		Mask:             route.Mask,
		NextHopIp:        route.NextHopIp,
		Prototype:        int(route.Prototype),
		NetworkStatement: route.NetworkStatement,
		RouteOrigin:      route.RouteOrigin,
	}
	return rv
}

func (mgr *FSRouteMgr) handleRibUpdates(rxBuf []byte) {
	var routeListInfo ribdCommonDefs.RoutelistInfo
	routes := make([]*config.RouteInfo, 0)
	reader := bytes.NewReader(rxBuf)
	decoder := json.NewDecoder(reader)
	msg := ribdCommonDefs.RibdNotifyMsg{}
	updateMsg := "Add"

	for err := decoder.Decode(&msg); err == nil; err = decoder.Decode(&msg) {
		err = json.Unmarshal(msg.MsgBuf, &routeListInfo)
		if err != nil {
			mgr.logger.Err(fmt.Sprintf(
				"Unmarshal RIB route update failed with err %s", err))
		}
		if msg.MsgType == ribdCommonDefs.NOTIFY_ROUTE_DELETED {
			updateMsg = "Remove"
		}
		mgr.logger.Info(fmt.Sprintln(updateMsg, "connected route, dest:",
			routeListInfo.RouteInfo.Ipaddr, "netmask:",
			routeListInfo.RouteInfo.Mask, "nexthop:",
			routeListInfo.RouteInfo.NextHopIp))
		route := mgr.populateConfigRoute(&routeListInfo.RouteInfo)
		routes = append(routes, route)
	}

	if len(routes) > 0 {
		if msg.MsgType == ribdCommonDefs.NOTIFY_ROUTE_CREATED {
			api.SendRouteNotification(routes, make([]*config.RouteInfo, 0))
		} else if msg.MsgType == ribdCommonDefs.NOTIFY_ROUTE_DELETED {
			api.SendRouteNotification(make([]*config.RouteInfo, 0), routes)
		} else {
			mgr.logger.Err(fmt.Sprintf("**** Received RIB update with ",
				"unknown type %d ****", msg.MsgType))
		}
	} else {
		mgr.logger.Err(fmt.Sprintf("**** Received RIB update type %d with no routes ****",
			msg.MsgType))
	}
}

func (mgr *FSRouteMgr) GetNextHopInfo(ipAddr string) (*config.NextHopInfo, error) {
	info, err := mgr.ribdClient.GetRouteReachabilityInfo(ipAddr)
	if err != nil {
		mgr.logger.Err(fmt.Sprintln("Getting route reachability for ",
			ipAddr, "failed, error:", err))
		return nil, err
	}
	reachInfo := &config.NextHopInfo{
		IPAddr:         info.Ipaddr,
		Mask:           info.Mask,
		Metric:         int32(info.Metric),
		NextHopIp:      info.NextHopIp,
		IsReachable:    info.IsReachable,
		NextHopIfIndex: int32(info.NextHopIfIndex),
	}
	return reachInfo, err
}

func (mgr *FSRouteMgr) createRibdIPv4RouteCfg(cfg *config.RouteConfig,
	create bool) *ribd.IPv4Route {
	rCfg := ribd.IPv4Route{
		Cost:              cfg.Cost,
		Protocol:          cfg.Protocol,
		NetworkMask:       cfg.NetworkMask,
		DestinationNw:     cfg.DestinationNw,
	}
	nextHop := ribd.NextHopInfo { 
		NextHopIp : cfg.NextHopIp,
		NextHopIntRef:cfg.OutgoingInterface,
	}
	rCfg.NextHop = make([]*ribd.NextHopInfo,0)
	rCfg.NextHop = append(rCfg.NextHop,&nextHop)
	return &rCfg
}

func (mgr *FSRouteMgr) CreateRoute(cfg *config.RouteConfig) {
	mgr.ribdClient.OnewayCreateIPv4Route(mgr.createRibdIPv4RouteCfg(cfg,
		true /*create*/))
}

func (mgr *FSRouteMgr) DeleteRoute(cfg *config.RouteConfig) {
	mgr.ribdClient.OnewayDeleteIPv4Route(mgr.createRibdIPv4RouteCfg(cfg,
		false /*delete*/))
}
func (mgr *FSRouteMgr) UpdateRoute(cfg *config.RouteConfig, op string) {
	rCfg := ribd.IPv4Route{
		Cost:              cfg.Cost,
		Protocol:          cfg.Protocol,
		NetworkMask:       cfg.NetworkMask,
		DestinationNw:     cfg.DestinationNw,
	}
	nextHop := ribd.NextHopInfo { 
		NextHopIp : cfg.NextHopIp,
		NextHopIntRef:cfg.OutgoingInterface,
	}
    rCfg.NextHop = make([]*ribd.NextHopInfo,0)
    rCfg.NextHop = append(rCfg.NextHop,&nextHop)
    value,err := json.Marshal(rCfg.NextHop)
	if err != nil {
		mgr.logger.Err(fmt.Sprintln("Err:", err, " while marshalling nexthop : ", rCfg.NextHop))
		return
	}
    patchOp := make([]*ribd.PatchOpInfo,0)
    patchOp = append(patchOp,&ribd.PatchOpInfo {
			        Op: op,
                     Path:"NextHop",
                     Value : string(value),
                     })
	mgr.ribdClient.UpdateIPv4Route(&rCfg, &rCfg, nil,patchOp)
}
func (mgr *FSRouteMgr) ApplyPolicy(protocol string, policy string, action string, conditions []*config.ConditionInfo) {
	temp := make([]ribdInt.ConditionInfo, len(conditions))
	ribdConditions := make([]*ribdInt.ConditionInfo, 0)
	j := 0
	for i := 0; i < len(conditions); i++ {
		temp[j] = ribdInt.ConditionInfo{conditions[i].ConditionType, conditions[i].Protocol, conditions[i].IpPrefix, conditions[i].MasklengthRange}
		ribdConditions = append(ribdConditions, &temp[j])
		j++
	}
	mgr.ribdClient.ApplyPolicy(protocol, policy, action, ribdConditions)
}

func (mgr *FSRouteMgr) GetRoutes() ([]*config.RouteInfo, []*config.RouteInfo) {
	var currMarker ribdInt.Int
	var count ribdInt.Int
	routes := make([]*config.RouteInfo, 0)
	count = 100
	for {
		mgr.logger.Info(fmt.Sprintln("Getting ", count,
			"objects from currMarker", currMarker))
		getBulkInfo, err := mgr.ribdClient.GetBulkRoutesForProtocol("BGP",
			currMarker, count)
		if err != nil {
			mgr.logger.Info(fmt.Sprintln("GetBulkRoutesForProtocol with err ", err))
			break
		}
		if getBulkInfo.Count == 0 {
			mgr.logger.Info("0 objects returned from GetBulkRoutesForProtocol")
			break
		}
		mgr.logger.Info(fmt.Sprintln("len(getBulkInfo.RouteList)  = ",
			len(getBulkInfo.RouteList), " num objects returned = ",
			getBulkInfo.Count))
		for idx, _ := range getBulkInfo.RouteList {
			route := mgr.populateConfigRoute(getBulkInfo.RouteList[idx])
			routes = append(routes, route)
		}
		if getBulkInfo.More == false {
			mgr.logger.Info("more returned as false, so no more get bulks")
			break
		}
		currMarker = ribdInt.Int(getBulkInfo.EndIdx)
	}
	if len(routes) == 0 {
		return nil, nil
	}

	return routes, (make([]*config.RouteInfo, 0))
}
