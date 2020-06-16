package heartbeat

import (
	"fmt"
	"github.com/lithdew/bytesutil"
	"github.com/valyala/bytebufferpool"
)

const FragmentHeaderSize uint8 = 5

const (
	RegularPacketPrefix byte = iota
	FragmentPacketPrefix
)

type fragmentHeader struct {
	prefix       byte
	seq          uint16
	fragmentID   uint8
	numFragments uint8
}

type fragmentReassemblyData struct {
	numFragmentsTotal    uint8
	numFragmentsReceived uint8
	fragmentReceived     []uint8
	size                 uint16
	packetData           []byte // TODO: allocate from bytebufferpool
}

func newFragmentRessembleyData(h *fragmentHeader, fragmentSize uint16) *fragmentReassemblyData {
	return &fragmentReassemblyData{
		numFragmentsTotal: h.numFragments,
		packetData:        make([]byte, uint16(h.numFragments)*fragmentSize),
	}
}

func (f *fragmentReassemblyData) reassemble(packet []byte, h *fragmentHeader, fragmentSize uint16) (*fragmentReassemblyData, bool) {
	f.numFragmentsReceived++
	f.fragmentReceived = append(f.fragmentReceived, h.fragmentID)
	copy(f.packetData[uint16(h.fragmentID)*fragmentSize:], packet)

	// last packet
	if h.fragmentID == h.numFragments-1 {
		f.size = uint16(f.numFragmentsTotal-1)*fragmentSize + uint16(len(packet))
	}

	// reassemble completed
	if f.numFragmentsReceived == f.numFragmentsTotal {
		return f, true
	}

	return f, false
}

func marshalFragmentHeader(b *bytebufferpool.ByteBuffer, seq uint16, fragmentID, numFragments uint8) {
	b.WriteByte(FragmentPacketPrefix)
	b.Write(bytesutil.AppendUint16BE(nil, seq))
	b.WriteByte(byte(fragmentID))
	b.WriteByte(byte(numFragments))
}

func unmarshalFragmentHeader(buf []byte) (*fragmentHeader, error) {
	if len(buf) < int(FragmentHeaderSize) {
		return nil, fmt.Errorf("fragment packet size is less than its header, pakcet: %v", buf)
	}

	if buf[0] != FragmentPacketPrefix {
		return nil, fmt.Errorf("packet prefix is wrong, prefix: %v", buf[0])
	}

	h := &fragmentHeader{}
	h.prefix = buf[0]
	h.seq = bytesutil.Uint16BE(buf[1:3])
	h.fragmentID = uint8(buf[3])
	h.numFragments = uint8(buf[4])

	if h.fragmentID >= h.numFragments {
		return nil, fmt.Errorf("fragment id is outside of range of num fragments, id: %v, num: %v", h.fragmentID, h.numFragments)
	}

	return h, nil
}
