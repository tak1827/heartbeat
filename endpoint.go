package heartbeat

import (
	"errors"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/lithdew/reliable"
	"github.com/valyala/bytebufferpool"
	// "log"
	"math"
	"net"
)

type receiveHandler func(packetData []byte)
type errorHandler func(err error)

type Endpoint struct {
	maxPacketSize uint16
	fragmentAbove uint16

	maxFragments uint16
	fragmentSize uint16

	conn *net.PacketConn
	addr net.Addr

	re *reliable.Endpoint

	pool bytebufferpool.Pool

	rh receiveHandler
	eh errorHandler

	fragmentReassembly map[uint16]*fragmentReassemblyData

	seq uint16
}

func NewEndpoint(conn net.PacketConn, rh receiveHandler, eh errorHandler, opts []Option, reOpts ...reliable.EndpointOption) (*Endpoint, error) {
	if conn == nil {
		return nil, errors.New("must path conn argument")
	}

	e := &Endpoint{
		addr:               conn.LocalAddr(),
		fragmentReassembly: make(map[uint16]*fragmentReassemblyData),
	}

	for _, opt := range opts {
		opt.apply(e)
	}

	if e.maxPacketSize == 0 {
		e.maxPacketSize = DefaultMaxPacketSize
	}

	if e.fragmentAbove == 0 {
		e.fragmentAbove = DefaultFragmentAbove
	}

	if e.maxFragments == 0 {
		e.maxFragments = DefaultMaxFragments
	}

	if e.fragmentSize == 0 {
		e.fragmentSize = DefaultFragmentSize
	}

	if rh != nil {
		e.rh = rh
	}

	if eh != nil {
		e.eh = eh
	}

	handler := func(buf []byte, addr net.Addr) {
		// ignore ack
		if len(buf) == 0 {
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
		extra      uint16
		fragmentID uint16
	)

	pBytes := len(packetData)
	if pBytes > int(e.maxPacketSize) {
		return fmt.Errorf("sending data is too large, size: %v", pBytes)
	}

	buf := e.pool.Get()
	defer e.pool.Put(buf)

	// regular packet
	if pBytes <= int(e.fragmentAbove) {
		buf.WriteByte(RegularPacketPrefix)
		buf.Write(packetData)

		msg := make([]byte, len(buf.B))
		copy(msg, buf.B)

		// log.Printf("%p: sending packet without fragmentation", e)

		e.re.WriteReliablePacket(msg, addr)

		return nil
	}

	// fragment packet
	if pBytes%int(e.fragmentSize) != 0 {
		extra = 1
	}
	numFragments := (uint16(pBytes) / e.fragmentSize) + extra

	// log.Printf("%p: sending packet (fragmentation=%v)", e, numFragments)

	// write each fragment with header and data
	for fragmentID = 0; fragmentID < numFragments; fragmentID++ {
		seq := e.seq % math.MaxUint16

		marshalFragmentHeader(buf, seq, fragmentID, numFragments)

		// last
		if fragmentID == numFragments-1 {
			buf.Write(packetData[fragmentID*e.fragmentSize:])
		} else {
			buf.Write(packetData[fragmentID*e.fragmentSize : (fragmentID+1)*e.fragmentSize])
		}

		msg := make([]byte, len(buf.B))
		copy(msg, buf.B)
		buf.Reset()

		e.re.WriteReliablePacket(msg, addr)
	}

	return nil
}

func (e *Endpoint) ReadPacket(packetData []byte) error {
	var (
		fPacket []byte
		data    *fragmentReassemblyData
		ok      bool
	)

	pBytes := len(packetData)
	if pBytes < 1 || pBytes > int(e.maxPacketSize)+int(FragmentHeaderBytes) {
		spew.Dump(packetData)
		return fmt.Errorf("packet size is out of range, size: %v", pBytes)
	}

	// regular packet
	if packetData[0] == byte(0) {
		if e.rh != nil {
			e.rh(packetData[1:])
		}
		return nil
	}

	// fragment packets
	h, err := unmarshalFragmentHeader(packetData)
	if err != nil {
		return err
	}

	fPacket = packetData[FragmentHeaderBytes:]

	if len(fPacket) > int(e.fragmentSize) {
		return fmt.Errorf("fragment size is out of range, size: %v", len(fPacket))
	}

	data, ok = e.fragmentReassembly[h.seq]
	if !ok {
		data = newFragmentRessembleyData(h, e.fragmentSize)
	}

	// ignore fragment already received
	if _, ok = searchUint8(h.fragmentID, data.fragmentReceived); ok {
		return nil
	}

	data.numFragmentsReceived++
	data.fragmentReceived = append(data.fragmentReceived, h.fragmentID)
	data.StoreFragmentData(h, e.fragmentSize, fPacket)

	// last
	if h.fragmentID == h.numFragments-1 {
		data.size = int(data.numFragmentsTotal-1)*int(e.fragmentSize) + len(packetData[FragmentHeaderBytes:])
	}

	// completed reassembly of packet
	if data.numFragmentsReceived == data.numFragmentsTotal {

		if e.rh != nil {
			e.rh(data.packetData[:data.size])
		}

		delete(e.fragmentReassembly, h.seq)

		return nil
	}

	e.fragmentReassembly[h.seq] = data

	return nil
}

func (e *Endpoint) Listen() {
	go e.re.Listen()
}

func (e *Endpoint) Close() error {
	return e.re.Close()
}