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

func moveFrontHeartbeats(e *Endpoint, target net.Addr) bool {
	var now, prev *Heartbeat

	for i := range e.hbs {
		now = e.hbs[i]
		e.hbs[i] = prev
		if now != nil && now.addr.String() == target.String() {
			e.hbs[0] = now
			return true
		}
		prev = now
		if prev == nil {
			return false
		}

		if i == len(e.hbs)-1 {
			e.hbs[i].deadline = time.Now()
		}
	}

	return false
}
