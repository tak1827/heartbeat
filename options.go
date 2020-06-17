package heartbeat

import (
	"net"
	"time"
)

const (
	// heartbeat options
	DefaultHeartbeatPeriod = 20 * time.Millisecond

	// fragment options
	DefaultFragmentSize uint32 = 4096
	DefaultMaxFragments uint8  = 64
)

type Option interface {
	apply(e *Endpoint)
}

type withHeartbeatPeriod struct{ heartbeatPeriod time.Duration }

func (o withHeartbeatPeriod) apply(e *Endpoint) { e.heartbeatPeriod = o.heartbeatPeriod }
func WithHeartbeatPeriod(heartbeatPeriod time.Duration) Option {
	return withHeartbeatPeriod{heartbeatPeriod: heartbeatPeriod}
}

type withFragmentSize struct{ fragmentSize uint32 }

func (o withFragmentSize) apply(e *Endpoint) { e.fragmentSize = o.fragmentSize }
func WithFragmentSize(fragmentSize uint32) Option {
	return withFragmentSize{fragmentSize: fragmentSize}
}

type withMaxFragments struct{ maxFragments uint8 }

func (o withMaxFragments) apply(e *Endpoint) { e.maxFragments = o.maxFragments }
func WithMaxFragments(maxFragments uint8) Option {
	return withMaxFragments{maxFragments: maxFragments}
}

func setDefaultOptions() *Endpoint {
	return &Endpoint{
		heartbeatPeriod: DefaultHeartbeatPeriod,
		fragmentSize:    DefaultFragmentSize,
		maxFragments:    DefaultMaxFragments,
		fReassembly:     make(map[uint16]*fragmentReassemblyData),
		exit:            make(chan struct{}),
		timers:          make(map[net.Addr]*time.Timer),
	}
}
