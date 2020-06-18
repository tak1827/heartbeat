package heartbeat

import (
	"time"
)

const (
	// heartbeat options
	DefaulthbPeriod                    = 20 * time.Millisecond
	DefaultHearbeatDeadline            = 1000 * time.Millisecond
	DefaultMaxHearbeatReceivers uint16 = 32

	// fragment options
	DefaultFragmentSize uint32 = 4096
	DefaultMaxFragments uint8  = 64
)

type Option interface {
	apply(e *Endpoint)
}

type withhbPeriod struct{ hbPeriod time.Duration }

func (o withhbPeriod) apply(e *Endpoint) { e.hbPeriod = o.hbPeriod }
func WithhbPeriod(hbPeriod time.Duration) Option {
	return withhbPeriod{hbPeriod: hbPeriod}
}

type withHearbeatTimeout struct{ hbDeadline time.Duration }

func (o withHearbeatTimeout) apply(e *Endpoint) { e.hbDeadline = o.hbDeadline }
func WithHearbeatTimeout(hbDeadline time.Duration) Option {
	return withHearbeatTimeout{hbDeadline: hbDeadline}
}

type withMaxHearbeatReceivers struct{ maxHbReceivers uint16 }

func (o withMaxHearbeatReceivers) apply(e *Endpoint) {
	e.maxHbReceivers = o.maxHbReceivers
}
func WithMaxHearbeatReceivers(maxHbReceivers uint16) Option {
	return withMaxHearbeatReceivers{maxHbReceivers: maxHbReceivers}
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
		hbPeriod:       DefaulthbPeriod,
		hbDeadline:     DefaultHearbeatDeadline,
		maxHbReceivers: DefaultMaxHearbeatReceivers,
		fragmentSize:   DefaultFragmentSize,
		maxFragments:   DefaultMaxFragments,
		rBuf:           initReassemblyBuf(),
		exit:           make(chan struct{}),
	}
}
