package heartbeat

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newEndpoint(t *testing.T, rh receiveHandler, eh errorHandler) (net.PacketConn, *Endpoint) {
	c, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)

	e, err := NewEndpoint(c, rh, eh, nil)
	require.NoError(t, err)

	return c, e
}

func TestWritePacket(t *testing.T) {
	defer goleak.VerifyNone(t)

	var mu sync.Mutex

	values := make(map[string]struct{})

	actual := uint64(0)
	expected := uint64(DefaultMaxPacketSize)

	recvHandler := func(buf []byte) {
		atomic.AddUint64(&actual, 1)

		mu.Lock()
		_, exists := values[string(buf)]
		delete(values, string(buf))
		mu.Unlock()

		require.True(t, exists)
	}

	errHandler := func(err error) {
		require.NoError(t, err)
	}

	ca, ea := newEndpoint(t, recvHandler, errHandler)
	cb, eb := newEndpoint(t, recvHandler, errHandler)

	defer func() {
		require.NoError(t, ca.SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, cb.SetDeadline(time.Now().Add(1*time.Millisecond)))

		require.NoError(t, ea.Close())
		require.NoError(t, eb.Close())

		ca.Close()
		cb.Close()
	}()

	go ea.Listen()
	go eb.Listen()

	for i := uint64(0); i < expected; i++ {
		data := bytes.Repeat([]byte("x"), int(i))

		mu.Lock()
		values[string(data)] = struct{}{}
		mu.Unlock()

		require.NoError(t, ea.WritePacket(data, eb.Addr()))
	}
}
