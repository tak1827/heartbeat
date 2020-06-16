package heartbeat

import (
	"net"
	"time"
)

const (
	// heartbeat options
	DefaultHeartbeatPeriod = 20 * time.Millisecond

	// fragment options
	DefaultFragmentSize uint16 = 1024
	DefaultMaxFragments uint16 = 64 - 1
)

type Option interface {
	apply(e *Endpoint)
}

type withHeartbeatPeriod struct{ heartbeatPeriod time.Duration }

func (o withHeartbeatPeriod) apply(e *Endpoint) { e.heartbeatPeriod = o.heartbeatPeriod }
func WithHeartbeatPeriod(heartbeatPeriod time.Duration) Option {
	return withHeartbeatPeriod{heartbeatPeriod: heartbeatPeriod}
}

type withFragmentSize struct{ fragmentSize uint16 }

func (o withFragmentSize) apply(e *Endpoint) { e.fragmentSize = o.fragmentSize }
func WithFragmentSize(fragmentSize uint16) Option {
	return withFragmentSize{fragmentSize: fragmentSize}
}

type withMaxFragments struct{ maxFragments uint16 }

func (o withMaxFragments) apply(e *Endpoint) { e.maxFragments = o.maxFragments }
func WithMaxFragments(maxFragments uint16) Option {
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
