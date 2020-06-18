package heartbeat

import (
	"fmt"
	"github.com/lithdew/bytesutil"
	"github.com/valyala/bytebufferpool"
)

const (
	FragmentHeaderSize uint8  = 5
	ReassemblyBufSize  uint16 = 512
)

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
	seq                  uint16
	numFragmentsTotal    uint8
	numFragmentsReceived uint8
	fragmentReceived     []uint8
	size                 uint32
	rBufIndex            uint16
}

func newFragmentRessembleyData(h *fragmentHeader) *fragmentReassemblyData {
	return &fragmentReassemblyData{
		seq:               h.seq,
		numFragmentsTotal: h.numFragments,
		rBufIndex:         h.seq % ReassemblyBufSize,
	}
}

func (f *fragmentReassemblyData) reassemble(e *Endpoint, h *fragmentHeader, packet []byte) (*fragmentReassemblyData, bool) {
	f.numFragmentsReceived++
	f.fragmentReceived = append(f.fragmentReceived, h.fragmentID)
	copy(e.rBuf[f.rBufIndex][uint32(h.fragmentID)*e.fragmentSize:], packet)

	// last packet
	if h.fragmentID == h.numFragments-1 {
		f.size = uint32(f.numFragmentsTotal-1)*e.fragmentSize + uint32(len(packet))
	}

	// reassemble completed
	if f.numFragmentsReceived == f.numFragmentsTotal {
		return f, true
	}

	return f, false
}

func initReassemblyBuf() (buf [ReassemblyBufSize][]byte) {
	for i := uint16(0); i < ReassemblyBufSize; i++ {
		buf[i] = make([]byte, int(DefaultFragmentSize)*int(DefaultMaxFragments))
	}
	return
}

func moveFrontReassemblyFragments(e *Endpoint, target uint16) bool {
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

func marshalFragmentHeader(b *bytebufferpool.ByteBuffer, seq uint16, fragmentID, numFragments uint8) {
	b.WriteByte(FragmentPacketPrefix)
	b.Write(bytesutil.AppendUint16BE(nil, seq))
	b.WriteByte(byte(fragmentID))
	b.WriteByte(byte(numFragments))
}

func unmarshalFragmentHeader(buf []byte) (*fragmentHeader, error) {
	if len(buf) < int(FragmentHeaderSize) {
		return nil, fmt.Errorf("fragment packet size is less than its header, pakcet: %+v", buf)
	}

	if buf[0] != FragmentPacketPrefix {
		return nil, fmt.Errorf("packet prefix is wrong, prefix: %+v", buf[0])
	}

	h := &fragmentHeader{}
	h.prefix = buf[0]
	h.seq = bytesutil.Uint16BE(buf[1:3])
	h.fragmentID = uint8(buf[3])
	h.numFragments = uint8(buf[4])

	if h.fragmentID >= h.numFragments {
		return nil, fmt.Errorf("fragment id is outside of range of num fragments, id: %+v, num: %+v", h.fragmentID, h.numFragments)
	}

	return h, nil
}
