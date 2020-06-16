package heartbeat

import (
	"time"
)

const (
	// heartbeat options
	DefaultHeartbeatPeriod = 20 * time.Millisecond

	// fragment options
	DefaultMaxPacketSize uint16 = 1024 * (64 - 1)
	DefaultFragmentAbove uint16 = 1024
	DefaultMaxFragments  uint16 = 16
	DefaultFragmentSize  uint16 = 1024
)

type Option interface {
	apply(e *Endpoint)
}

type withHeartbeatPeriod struct{ heartbeatPeriod time.Duration }

func (o withHeartbeatPeriod) apply(e *Endpoint) { e.heartbeatPeriod = o.heartbeatPeriod }
func WithHeartbeatPeriod(heartbeatPeriod time.Duration) Option {
	return withHeartbeatPeriod{heartbeatPeriod: heartbeatPeriod}
}

type withMaxPacketSize struct{ maxPacketSize uint16 }

func (o withMaxPacketSize) apply(e *Endpoint) { e.maxPacketSize = o.maxPacketSize }
func WithMaxPacketSize(maxPacketSize uint16) Option {
	return withMaxPacketSize{maxPacketSize: maxPacketSize}
}

type withFragmentAbove struct{ fragmentAbove uint16 }

func (o withFragmentAbove) apply(e *Endpoint) { e.fragmentAbove = o.fragmentAbove }
func WithFragmentAbove(fragmentAbove uint16) Option {
	return withFragmentAbove{fragmentAbove: fragmentAbove}
}

type withMaxFragments struct{ maxFragments uint16 }

func (o withMaxFragments) apply(e *Endpoint) { e.maxFragments = o.maxFragments }
func WithMaxFragments(maxFragments uint16) Option {
	return withMaxFragments{maxFragments: maxFragments}
}

type withFragmentSize struct{ fragmentSize uint16 }

func (o withFragmentSize) apply(e *Endpoint) { e.fragmentSize = o.fragmentSize }
func WithFragmentSize(fragmentSize uint16) Option {
	return withFragmentSize{fragmentSize: fragmentSize}
}
