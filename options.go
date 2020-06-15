package heartbeat

import (
	"time"
)

const (
	DefaultHeartbeatPeriod = 20 * time.Millisecond

	DefaultMaxPacketSize uint16 = 1024 * 63

	DefaultFragmentAbove uint16 = 1024
	DefaultMaxFragments  uint16 = 16
	DefaultFragmentSize  uint16 = 1024
)

type Option interface {
	apply(e *Endpoint)
}

type withMaxPacketSize struct{ maxPacketSize uint16 }

func (o withMaxPacketSize) apply(e *Endpoint) { e.maxPacketSize = o.maxPacketSize }
func WithMaxPacketSize(maxPacketSize uint16) Option {
	return withMaxPacketSize{maxPacketSize: maxPacketSize}
}
