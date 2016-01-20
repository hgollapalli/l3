// peer.go
package server

import (
	"fmt"
	"l3/bgp/config"
	"l3/bgp/packet"
	"log/syslog"
	"net"
	"sync"
)

type CONFIG int

const (
	START CONFIG = iota
	STOP
)

type BgpPkt struct {
	connDir config.ConnDir
	pkt     *packet.BGPMessage
}

type FSMManager struct {
	Peer        *Peer
	logger      *syslog.Writer
	gConf       *config.GlobalConfig
	pConf       *config.NeighborConfig
	fsms        map[uint8]*FSM
	acceptCh    chan net.Conn
	acceptErrCh chan error
	closeCh     chan bool
	acceptConn  bool
	commandCh   chan int
	activeFSM   uint8
	newConnCh   chan PeerFSMConnState
	fsmMutex    sync.RWMutex
}

func NewFSMManager(peer *Peer, globalConf *config.GlobalConfig, peerConf *config.NeighborConfig) *FSMManager {
	mgr := FSMManager{
		Peer:   peer,
		logger: peer.logger,
		gConf:  globalConf,
		pConf:  peerConf,
	}
	mgr.fsms = make(map[uint8]*FSM)
	mgr.acceptCh = make(chan net.Conn)
	mgr.acceptErrCh = make(chan error)
	mgr.acceptConn = false
	mgr.closeCh = make(chan bool)
	mgr.commandCh = make(chan int)
	mgr.activeFSM = uint8(config.ConnDirInvalid)
	mgr.newConnCh = make(chan PeerFSMConnState)
	mgr.fsmMutex = sync.RWMutex{}
	return &mgr
}

func (mgr *FSMManager) Init() {
	fsmId := uint8(config.ConnDirOut)
	fsm := NewFSM(mgr, fsmId, mgr.Peer)
	go fsm.StartFSM(NewIdleState(fsm))
	mgr.fsms[fsmId] = fsm
	fsm.passiveTcpEstCh <- true

	for {
		select {
		case inConn := <-mgr.acceptCh:
			mgr.logger.Info(fmt.Sprintf("Neighbor %s: Received a connection OPEN from far end",
				mgr.pConf.NeighborAddress))
			if !mgr.acceptConn {
				mgr.logger.Info(fmt.Sprintln("Can't accept connection from ", mgr.pConf.NeighborAddress,
					"yet."))
				inConn.Close()
			} else {
				foundInConn := false
				for _, fsm = range mgr.fsms {
					if fsm != nil && fsm.peerConn != nil && fsm.peerConn.dir == config.ConnDirIn {
						mgr.logger.Info(fmt.Sprintln("A FSM is already created for a incoming connection"))
						foundInConn = true
						break
					}
				}
				if !foundInConn {
					for _, fsm = range mgr.fsms {
						if fsm != nil {
							fsm.inConnCh <- inConn
						}
					}
				}
			}

		case <-mgr.acceptErrCh:
			mgr.logger.Info(fmt.Sprintf("Neighbor %s: Received a connection CLOSE from far end",
				mgr.pConf.NeighborAddress))
			for _, fsm := range mgr.fsms {
				if fsm != nil && fsm.peerConn != nil && fsm.peerConn.dir == config.ConnDirIn {
					fsm.eventRxCh <- BGPEventTcpConnFails
				}
			}

		case newConn := <-mgr.newConnCh:
			mgr.logger.Info(fmt.Sprintf("FSMManager: Neighbor %s FSM %d Handle another connection",
				mgr.pConf.NeighborAddress, newConn.id))
			newId := mgr.getNewId(newConn.id)
			mgr.handleAnotherConnection(newId, newConn.connDir, newConn.conn)

		case <-mgr.closeCh:
			mgr.Cleanup()
			return

		case command := <-mgr.commandCh:
			event := BGPFSMEvent(command)
			if (event == BGPEventManualStart) || (event == BGPEventManualStop) ||
				(event == BGPEventManualStartPassTcpEst) {
				for _, fsm := range mgr.fsms {
					if fsm != nil {
						fsm.eventRxCh <- event
					}
				}
			}
		}
	}
}

func (mgr *FSMManager) AcceptPeerConn() {
	mgr.acceptConn = true
}

func (mgr *FSMManager) RejectPeerConn() {
	mgr.acceptConn = false
}

func (mgr *FSMManager) fsmEstablished(id uint8, conn *net.Conn) {
	mgr.logger.Info(fmt.Sprintf("FSMManager: Peer %s FSM %d connection established", mgr.pConf.NeighborAddress.String(), id))
	mgr.activeFSM = id
	mgr.Peer.PeerConnEstablished(conn)
}

func (mgr *FSMManager) fsmBroken(id uint8, fsmDelete bool) {
	mgr.logger.Info(fmt.Sprintf("FSMManager: Peer %s FSM %d connection broken", mgr.pConf.NeighborAddress.String(), id))
	if mgr.activeFSM == id {
		mgr.activeFSM = uint8(config.ConnDirInvalid)
	}

	mgr.Peer.PeerConnBroken(fsmDelete)
}

func (mgr *FSMManager) SendUpdateMsg(bgpMsg *packet.BGPMessage) {
	defer mgr.fsmMutex.Unlock()
	mgr.fsmMutex.Lock()

	if mgr.activeFSM == uint8(config.ConnDirInvalid) {
		mgr.logger.Info(fmt.Sprintf("FSMManager: Neighbor %s FSM is not in ESTABLISHED state", mgr.pConf.NeighborAddress))
		return
	}
	mgr.logger.Info(fmt.Sprintf("FSMManager: Neighbor %s FSM %d - send update", mgr.pConf.NeighborAddress, mgr.activeFSM))
	mgr.fsms[mgr.activeFSM].pktTxCh <- bgpMsg
}

func (mgr *FSMManager) Cleanup() {
	defer mgr.fsmMutex.Unlock()
	mgr.fsmMutex.Lock()

	for id, fsm := range mgr.fsms {
		if fsm != nil {
			mgr.logger.Info(fmt.Sprintf("FSMManager: Neighbor %s FSM %d - cleanup FSM", mgr.pConf.NeighborAddress, id))
			fsm.closeCh <- true
			fsm = nil
			mgr.fsmBroken(id, true)
			mgr.fsms[id] = nil
			delete(mgr.fsms, id)
		}
	}
}

func (mgr *FSMManager) getNewId(id uint8) uint8 {
	return uint8((id + 1) % 2)
}

func (mgr *FSMManager) handleAnotherConnection(id uint8, connDir config.ConnDir, conn *net.Conn) {
	defer mgr.fsmMutex.Unlock()
	mgr.fsmMutex.Lock()

	if mgr.fsms[id] != nil {
		mgr.logger.Err(fmt.Sprintf("FSMManager: Neighbor %s - FSM with id %d already exists", mgr.pConf.NeighborAddress, id))
		return
	}

	mgr.logger.Info(fmt.Sprintf("FSMManager: Neighbor %s Creating new FSM with id %d", mgr.pConf.NeighborAddress, id))
	fsm := NewFSM(mgr, id, mgr.Peer)

	var state BaseStateIface
	state = NewActiveState(fsm)
	connCh := fsm.inConnCh
	if connDir == config.ConnDirOut {
		state = NewConnectState(fsm)
		connCh = fsm.outConnCh
	}
	mgr.fsms[id] = fsm
	go fsm.StartFSM(state)
	connCh <- *conn
	fsm.passiveTcpEstCh <- true
}

func (mgr *FSMManager) getFSMIdByDir(connDir config.ConnDir) uint8 {
	for id, fsm := range mgr.fsms {
		if fsm != nil && fsm.peerConn != nil && fsm.peerConn.dir == connDir {
			return id
		}
	}

	return uint8(config.ConnDirInvalid)
}

func (mgr *FSMManager) receivedBGPOpenMessage(id uint8, connDir config.ConnDir, openMsg *packet.BGPOpen) {
	var closeConnDir config.ConnDir = config.ConnDirInvalid

	defer mgr.fsmMutex.Unlock()
	mgr.fsmMutex.Lock()

	localBGPId := packet.ConvertIPBytesToUint(mgr.gConf.RouterId.To4())
	bgpIdInt := packet.ConvertIPBytesToUint(openMsg.BGPId.To4())
	for fsmId, fsm := range mgr.fsms {
		if fsmId != id && fsm != nil && fsm.State.state() >= BGPFSMOpensent {
			if fsm.State.state() == BGPFSMEstablished {
				closeConnDir = connDir
			} else if localBGPId > bgpIdInt {
				closeConnDir = config.ConnDirIn
			} else {
				closeConnDir = config.ConnDirOut
			}
			closeFSMId := mgr.getFSMIdByDir(closeConnDir)
			if closeFSM, ok := mgr.fsms[closeFSMId]; ok {
				mgr.logger.Info(fmt.Sprintf("FSMManager: Peer %s, close FSM %d", mgr.pConf.NeighborAddress.String(), closeFSMId))
				closeFSM.closeCh <- true
				mgr.fsmBroken(closeFSMId, false)
				mgr.fsms[closeFSMId] = nil
				delete(mgr.fsms, closeFSMId)
			}
		}
	}
	if closeConnDir == config.ConnDirInvalid || closeConnDir != connDir {
		asSize := packet.GetASSize(openMsg)
		mgr.Peer.SetPeerAttrs(openMsg.BGPId, asSize)
	}
}
