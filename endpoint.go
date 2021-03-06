package heartbeat

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/lithdew/reliable"
	"github.com/valyala/bytebufferpool"
	// "log"
	"math"
	"net"
	"sync"
	"time"
)

type receiveHandler func(packetData []byte)
type errorHandler func(err error)

type Endpoint struct {
	// heartbeat options
	hbPeriod       time.Duration
	hbDeadline     time.Duration
	maxHbReceivers uint16

	// fragments options
	fragmentSize uint32
	maxFragments uint8

	conn *net.PacketConn
	addr net.Addr

	re *reliable.Endpoint

	seq uint16 // fragment packets sequence number

	hbs []*Heartbeat

	// Note: last reassembling fragments are removed, when new one come
	rFragments [ReassemblyBufSize]*fragmentReassemblyData // reassembling fragment packet list
	rBuf       [ReassemblyBufSize][]byte                  // packet data of reassembling fragment packets

	// Note: ack and heartbeat are ignored on this receiver
	rh receiveHandler
	eh errorHandler

	exit chan struct{} // signal channel to close the conn

	pool bytebufferpool.Pool

	mu sync.Mutex
}

// Note: about reOpts, reliable.WithEndpointPacketHandler is overwritten
func NewEndpoint(conn net.PacketConn, rh receiveHandler, eh errorHandler, opts []Option, reOpts ...reliable.EndpointOption) (*Endpoint, error) {
	if conn == nil {
		return nil, errors.New("no conn argument")
	}

	e := setDefaultOptions()

	e.addr = conn.LocalAddr()

	for _, opt := range opts {
		opt.apply(e)
	}

	e.hbs = make([]*Heartbeat, e.maxHbReceivers)

	maxPacketSize := int(e.fragmentSize) * int(e.maxFragments)
	if maxPacketSize > math.MaxUint32 {
		return nil, fmt.Errorf("max packet size must be lower than MaxUint32 size: %+v", maxPacketSize)
	}

	if rh != nil {
		e.rh = rh
	}

	if eh != nil {
		e.eh = eh
	}

	handler := func(buf []byte, addr net.Addr) {
		// ignore ack and hearbeat
		if len(buf) == 0 || bytes.Compare(buf, append([]byte{0}, HeartbeatMsg...)) == 0 {
			return
		}

		if err := e.ReadPacket(buf); err != nil {
			if e.eh != nil {
				e.eh(err)
			}
		}
	}

	e.re = reliable.NewEndpoint(conn, append(reOpts, reliable.WithEndpointPacketHandler(handler))...)

	return e, nil
}

func (e *Endpoint) Addr() net.Addr {
	return e.addr
}

func (e *Endpoint) WritePacket(packetData []byte, addr net.Addr) error {
	var (
		extra      uint8
		fragmentID uint8
	)

	pSize := len(packetData)
	if pSize > int(e.fragmentSize)*int(e.maxFragments) {
		return fmt.Errorf("sending data is too large, size: %+v", pSize)
	}

	buf := e.pool.Get()
	defer e.pool.Put(buf)

	if pSize <= int(e.fragmentSize) {
		// regular packet
		buf.WriteByte(RegularPacketPrefix)
		buf.Write(packetData)

		// log.Printf("%p: sending packet without fragmentation (size=%+v)", e, len(packetData))

		e.re.WriteReliablePacket(buf.B, addr)

	} else {
		// fragment packet
		if pSize%int(e.fragmentSize) != 0 {
			extra = 1
		}
		numFragments := uint8(pSize/int(e.fragmentSize)) + extra

		// log.Printf("%p: sending packet (fragmentation=%+v) (size=%+v)", e, numFragments, len(packetData))

		e.mu.Lock()
		seq := e.seq % math.MaxUint16
		e.seq++
		e.mu.Unlock()

		// write each fragment with header and data
		// Note: avoid concurrently sending which lead to slow down
		for fragmentID = 0; fragmentID < numFragments; fragmentID++ {
			marshalFragmentHeader(buf, seq, fragmentID, numFragments)

			if fragmentID == numFragments-1 {
				// last　fragment
				buf.Write(packetData[uint32(fragmentID)*e.fragmentSize:])
			} else {
				buf.Write(packetData[uint32(fragmentID)*e.fragmentSize : uint32(fragmentID+1)*e.fragmentSize])
			}

			e.re.WriteReliablePacket(buf.B, addr)

			buf.Reset()
		}
	}

	// run heartbeat if not hearbeat packet
	if bytes.Compare(packetData, HeartbeatMsg) != 0 {
		go e.runHeartbeat(addr)
	}

	return nil
}

func (e *Endpoint) ReadPacket(packetData []byte) error {
	var (
		data      *fragmentReassemblyData
		completed bool
	)

	// log.Printf("%p: recv packet (size=%+v)", e, len(packetData))

	pSize := len(packetData)
	if pSize < 1 || pSize > int(e.fragmentSize)+int(FragmentHeaderSize) {
		return fmt.Errorf("packet size is out of range, size: %+v", pSize)
	}

	// regular packet
	if packetData[0] == byte(0) {
		if e.rh != nil {
			e.rh(packetData[1:])
		}
		return nil
	}

	// fragment packets
	h, err := unmarshalFragmentHeader(packetData[:FragmentHeaderSize])
	if err != nil {
		return err
	}

	if len(packetData[FragmentHeaderSize:]) > int(e.fragmentSize) {
		return fmt.Errorf("fragment size is out of range, size: %+v", len(packetData[FragmentHeaderSize:]))
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	exist := moveFrontReassemblyFragments(e, h.seq)
	if exist {
		data = e.rFragments[0]
	} else {
		data = newFragmentRessembleyData(h)
	}

	// ignore already received fragment
	if _, ok := searchUint8(h.fragmentID, data.fragmentReceived); ok {
		return nil
	}

	data, completed = data.reassemble(e, h, packetData[FragmentHeaderSize:])
	if completed {
		if e.rh != nil {
			e.rh(e.rBuf[data.rBufIndex][:data.size])
		}
		return nil
	}

	e.rFragments[0] = data

	return nil
}

func (e *Endpoint) Listen() {
	e.re.Listen()
}

func (e *Endpoint) Close() error {
	if err := e.re.Close(); err != nil {
		return err
	}
	close(e.exit)
	return nil
}

func (e *Endpoint) runHeartbeat(addr net.Addr) {
	e.mu.Lock()
	defer e.mu.Unlock()

	exist := moveFrontHeartbeats(e, addr)
	if !exist {
		e.hbs[0] = newHeartbeat(addr, e.hbPeriod, e.hbDeadline)
	} else {
		// reset heartbeat timer
		e.hbs[0].timer.Reset(e.hbPeriod)
		e.hbs[0].deadline = time.Now().Add(e.hbDeadline)
	}

	if e.hbs[0].stopped {
		// run heartbeat
		go func(hb *Heartbeat) {
			e.sendHeartbeat(hb)
		}(e.hbs[0])
	}
}

func (e *Endpoint) sendHeartbeat(hb *Heartbeat) {
	for {
		select {
		case <-e.exit:
			return
		case <-hb.timer.C:
			e.mu.Lock()
			if hb.stopped || time.Now().After(hb.deadline) {
				hb.stopped = true
				e.mu.Unlock()
				return
			}
			hb.timer.Reset(e.hbPeriod)
			e.mu.Unlock()
			if err := e.WritePacket(HeartbeatMsg, hb.addr); err != nil {
				if e.eh != nil {
					e.eh(fmt.Errorf("failed to write heartbeat, %+v", err))
				}
			}
		}
	}
}
