package heartbeat

import (
	"net"
	"time"
)

var HeartbeatMsg = []byte(".h")

type Heartbeat struct {
	addr     net.Addr
	timer    *time.Timer
	deadline time.Time
	stopped  bool
}

func newHeartbeat(addr net.Addr, period, deadline time.Duration) *Heartbeat {
	return &Heartbeat{
		addr:     addr,
		timer:    time.NewTimer(period),
		deadline: time.Now().Add(deadline),
		stopped:  true,
	}
}
