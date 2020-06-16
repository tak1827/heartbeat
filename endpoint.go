package heartbeat

import (
	"bytes"
	"errors"
	"fmt"
	// "github.com/davecgh/go-spew/spew"
	"github.com/lithdew/reliable"
	"github.com/valyala/bytebufferpool"
	// "log"
	"math"
	"net"
	"sync"
	"time"
)

var HeartbeatMsg = []byte("heartbeat")

type receiveHandler func(packetData []byte)
type errorHandler func(err error)

type Endpoint struct {
	// heartbeat options
	heartbeatPeriod time.Duration

	// fragments options
	fragmentSize uint16
	maxFragments uint16

	conn *net.PacketConn
	addr net.Addr

	re *reliable.Endpoint

	seq uint16 // fragment packets sequence number

	timers map[net.Addr]*time.Timer // heartbeat timer

	// TODO: garbage collection
	fReassembly map[uint16]*fragmentReassemblyData // map for reassembling fragment packets

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

	reOpts = append(reOpts, reliable.WithEndpointPacketHandler(handler))

	e.re = reliable.NewEndpoint(conn, reOpts...)

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
	if pSize > int(e.fragmentSize*e.maxFragments) {
		return fmt.Errorf("sending data is too large, size: %v", pSize)
	}

	buf := e.pool.Get()
	defer e.pool.Put(buf)

	if pSize <= int(e.fragmentSize) {
		// regular packet
		buf.WriteByte(RegularPacketPrefix)
		buf.Write(packetData)

		// log.Printf("%p: sending packet without fragmentation", e)

		e.re.WriteReliablePacket(buf.B, addr)

	} else {
		// fragment packet
		if pSize%int(e.fragmentSize) != 0 {
			extra = 1
		}
		numFragments := uint8(pSize/int(e.fragmentSize)) + extra

		// log.Printf("%p: sending packet (fragmentation=%v)", e, numFragments)

		// write each fragment with header and data
		for fragmentID = 0; fragmentID < numFragments; fragmentID++ {
			seq := e.seq % math.MaxUint16

			marshalFragmentHeader(buf, seq, fragmentID, numFragments)

			if fragmentID == numFragments-1 {
				// last　fragment
				buf.Write(packetData[uint16(fragmentID)*e.fragmentSize:])
			} else {
				buf.Write(packetData[uint16(fragmentID)*e.fragmentSize : uint16(fragmentID+1)*e.fragmentSize])
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
		fPacket       []byte
		data          *fragmentReassemblyData
		ok, completed bool
	)

	pSize := len(packetData)
	if pSize < 1 || pSize > int(e.fragmentSize)+int(FragmentHeaderSize) {
		return fmt.Errorf("packet size is out of range, size: %v", pSize)
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

	fPacket = packetData[FragmentHeaderSize:]

	if len(fPacket) > int(e.fragmentSize) {
		return fmt.Errorf("fragment size is out of range, size: %v", len(fPacket))
	}

	data, ok = e.fReassembly[h.seq]
	if !ok {
		data = newFragmentRessembleyData(h, e.fragmentSize)
	}

	// ignore already received fragment
	if _, ok = searchUint8(h.fragmentID, data.fragmentReceived); ok {
		return nil
	}

	data, completed = data.reassemble(fPacket, h, e.fragmentSize)
	if completed {
		if e.rh != nil {
			e.rh(data.packetData[:data.size])
		}

		delete(e.fReassembly, h.seq)

		return nil
	}

	e.fReassembly[h.seq] = data

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
	e.wg.Wait()
	return nil
}

func (e *Endpoint) runHeartbeat(addr net.Addr) {
	e.mu.Lock()
	defer e.mu.Unlock()

	timer := e.timers[addr]
	if timer == nil {
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
					e.eh(fmt.Errorf("failed to write heartbeat, %v", err))
				}
			}
		}
	}
}
