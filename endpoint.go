package heartbeat

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/lithdew/reliable"
	"github.com/valyala/bytebufferpool"
	// "log"
	// "github.com/davecgh/go-spew/spew"
	"math"
	"net"
	"sync"
	"time"
)

// NOTE: a data same as "HeartbeatMsg" is ignored
var HeartbeatMsg = []byte(".h")

type receiveHandler func(packetData []byte)
type errorHandler func(err error)

type Endpoint struct {
	// heartbeat options
	heartbeatPeriod time.Duration

	// fragments options
	fragmentSize uint32
	maxFragments uint8

	conn *net.PacketConn
	addr net.Addr

	re *reliable.Endpoint

	seq uint16 // fragment packets sequence number

	timers map[net.Addr]*time.Timer // heartbeat timer

	// Note: last reassembling fragments are removed, when new one come
	rFragments [ReassemblyBufSize]*fragmentReassemblyData // reassembling fragment packet list
	rBuf       [ReassemblyBufSize][]byte                  // packet data of reassembling fragment packets

	exit chan struct{} // signal channel to close the conn

	// Note: ack and heartbeat are ignored on this receiver
	rh receiveHandler
	eh errorHandler

	pool bytebufferpool.Pool

	mu sync.Mutex
	wg sync.WaitGroup
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

	e.runHeartbeat(addr)

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

	// fPacket = packetData[FragmentHeaderSize:]

	if len(packetData[FragmentHeaderSize:]) > int(e.fragmentSize) {
		return fmt.Errorf("fragment size is out of range, size: %+v", len(packetData[FragmentHeaderSize:]))
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	exist := e.moveFrontReassemblyFragments(h.seq)
	if exist {
		data = e.rFragments[0]
	} else {
		data = newFragmentRessembleyData(h)
	}

	// ignore already received fragment

	if _, ok := searchUint8(h.fragmentID, data.fragmentReceived); ok {
		return nil
	}

	e.reassemble(packetData[FragmentHeaderSize:], data.rBufIndex, h.fragmentID)

	data, completed = data.update(h, e.fragmentSize, len(packetData[FragmentHeaderSize:]))
	if completed {
		if e.rh != nil {
			e.rh(e.rBuf[data.rBufIndex][:data.size])
		}
		return nil
	}

	e.rFragments[0] = data

	return nil
}

func (e *Endpoint) reassemble(packet []byte, i uint16, fragmentID uint8) {
	copy(e.rBuf[i][uint32(fragmentID)*e.fragmentSize:], packet)
}

func (e *Endpoint) moveFrontReassemblyFragments(target uint16) bool {
	var now, prev *fragmentReassemblyData

	for i := range e.rFragments {
		now = e.rFragments[i]
		e.rFragments[i] = prev
		if now != nil && now.seq == target {
			e.rFragments[0] = now
			return true
		}
		prev = now
		if prev == nil {
			return false
		}
	}

	return false
}

func (e *Endpoint) Listen() {
	e.re.Listen()
}

func (e *Endpoint) Close() error {
	if err := e.re.Close(); err != nil {
		return err
	}
	close(e.exit)
	e.wg.Wait()
	return nil
}

func (e *Endpoint) runHeartbeat(addr net.Addr) {
	e.mu.Lock()
	defer e.mu.Unlock()

	timer, exist := e.timers[addr]
	if !exist {
		// run heartbeat
		timer = time.NewTimer(e.heartbeatPeriod)
		e.timers[addr] = timer

		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			e.sendHeartbeat(timer, addr)
		}()

	} else {
		// reset heartbeat timer
		timer.Reset(e.heartbeatPeriod)
	}
}

func (e *Endpoint) sendHeartbeat(timer *time.Timer, addr net.Addr) {
	for {
		select {
		case <-e.exit:
			return
		case <-timer.C:
			timer.Reset(e.heartbeatPeriod)
			if err := e.WritePacket(HeartbeatMsg, addr); err != nil {
				if e.eh != nil {
					e.eh(fmt.Errorf("failed to write heartbeat, %+v", err))
				}
			}
		}
	}
}
