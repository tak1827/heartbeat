package heartbeat

import (
	"fmt"
	"github.com/lithdew/bytesutil"
	"github.com/valyala/bytebufferpool"
)

const FragmentHeaderBytes uint8 = 5

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
	size                 int
	packetData           []byte // TODO: allocate from bytebufferpool
}

func newFragmentRessembleyData(h *fragmentHeader, fragmentSize uint16) *fragmentReassemblyData {
	return &fragmentReassemblyData{
		numFragmentsTotal: h.numFragments,
		packetData:        make([]byte, uint16(h.numFragments)*fragmentSize),
	}
}

func (f *fragmentReassemblyData) StoreFragmentData(h *fragmentHeader, fragmentSize uint16, packet []byte) {
	copy(f.packetData[uint16(h.fragmentID)*fragmentSize:], packet)
}

func marshalFragmentHeader(b *bytebufferpool.ByteBuffer, seq, fragmentID, numFragments uint16) {
	b.WriteByte(FragmentPacketPrefix)
	b.Write(bytesutil.AppendUint16BE(nil, seq))
	b.WriteByte(byte(fragmentID))
	b.WriteByte(byte(numFragments))
}

func unmarshalFragmentHeader(buf []byte) (*fragmentHeader, error) {
	if len(buf) < int(FragmentHeaderBytes) {
		return nil, fmt.Errorf("fragment packet size is less than its header, pakcet: %v", buf)
	}

	h := &fragmentHeader{}
	h.prefix, buf = buf[0], buf[1:]
	h.seq, buf = bytesutil.Uint16BE(buf[:2]), buf[2:]
	h.fragmentID, buf = uint8(buf[0]), buf[1:]
	h.numFragments, buf = uint8(buf[0]), buf[1:]

	if h.prefix != FragmentPacketPrefix {
		return nil, fmt.Errorf("packet prefix is wrong, prefix: %v", buf[0])
	}

	if h.fragmentID >= h.numFragments {
		return nil, fmt.Errorf("fragment id is outside of range of num fragments, id: %v, num: %v", h.fragmentID, h.numFragments)
	}

	return h, nil
}
