package server

import (
	"asicd/asicdCommonDefs"
	"errors"
	"fmt"
	"utils/commonDefs"
)

func (server *ARPServer) processResolveIPv4(conf ResolveIPv4) {
	server.logger.Debug(fmt.Sprintln("Received ResolveIPv4 call for TargetIP:", conf.TargetIP, "ifIndex:", conf.IfId))
	if conf.TargetIP == "0.0.0.0" {
		return
	}
	IfIndex := conf.IfId
	ifType := asicdCommonDefs.GetIntfIdFromIfIndex(int32(IfIndex))
	if ifType == commonDefs.IfTypeVlan {
		vlanEnt := server.vlanPropMap[IfIndex]
		for port, _ := range vlanEnt.UntagPortMap {
			server.arpEntryUpdateCh <- UpdateArpEntryMsg{
				PortNum: port,
				IpAddr:  conf.TargetIP,
				MacAddr: "incomplete",
				Type:    true,
			}
			server.sendArpReq(conf.TargetIP, port)
		}
	} else if ifType == commonDefs.IfTypeLag {
		lagEnt := server.lagPropMap[IfIndex]
		for port, _ := range lagEnt.PortMap {
			server.arpEntryUpdateCh <- UpdateArpEntryMsg{
				PortNum: port,
				IpAddr:  conf.TargetIP,
				MacAddr: "incomplete",
				Type:    true,
			}
			server.sendArpReq(conf.TargetIP, port)
		}
	} else if ifType == commonDefs.IfTypePort {
		server.arpEntryUpdateCh <- UpdateArpEntryMsg{
			PortNum: IfIndex,
			IpAddr:  conf.TargetIP,
			MacAddr: "incomplete",
			Type:    true,
		}
		server.sendArpReq(conf.TargetIP, IfIndex)
	}
}

func (server *ARPServer) processDeleteResolvedIPv4(ipAddr string) {
	server.logger.Info(fmt.Sprintln("Delete Resolved IPv4 for ipAddr:", ipAddr))
	server.arpDeleteArpEntryFromRibCh <- ipAddr
}

func (server *ARPServer) processArpConf(conf ArpConf) (int, error) {
	server.logger.Debug(fmt.Sprintln("Received ARP Timeout Value via Configuration:", conf.RefTimeout))
	if conf.RefTimeout < server.minRefreshTimeout {
		server.logger.Err(fmt.Sprintln("Refresh Timeout is below minimum allowed refresh timeout value of:", server.minRefreshTimeout))
		err := errors.New("Invalid Timeout Value")
		return 0, err
	} else if conf.RefTimeout == server.confRefreshTimeout {
		server.logger.Err(fmt.Sprintln("Arp is already configured with Refresh Timeout Value of:", server.confRefreshTimeout, "(seconds)"))
		return 0, nil
	}

	server.timeoutCounter = conf.RefTimeout / server.timerGranularity
	server.arpEntryCntUpdateCh <- server.timeoutCounter
	return 0, nil
}

func (server *ARPServer) processArpAction(msg ArpActionMsg) {
	server.logger.Info(fmt.Sprintln("Processing Arp Action msg", msg))
	server.arpActionProcessCh <- msg
}
