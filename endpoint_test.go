package heartbeat

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	// "github.com/davecgh/go-spew/spew"
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

	values := make(map[int]struct{})

	actual := uint32(0)
	expected := uint32(1024)
	multipleFactor := 100

	recvHandler := func(buf []byte) {
		atomic.AddUint32(&actual, 1)

		mu.Lock()
		_, exists := values[len(buf)]
		delete(values, len(buf))
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

		require.EqualValues(t, expected, atomic.LoadUint32(&actual))
	}()

	go ea.Listen()
	go eb.Listen()

	for i := uint32(0); i < expected; i++ {
		data := bytes.Repeat([]byte("x"), int(i)*multipleFactor)

		mu.Lock()
		values[len(data)] = struct{}{}
		mu.Unlock()

		require.NoError(t, ea.WritePacket(data, eb.Addr()))
	}
}

func TestHertbeat(t *testing.T) {
	defer goleak.VerifyNone(t)

	waitPeriod := 3 * time.Second

	sentMsg := []byte("data")
	actualSentMsg := uint32(0)
	expectedSentMsg := uint32(11)

	recvHandler := func(buf []byte) {
		if bytes.Compare(buf, sentMsg) == 0 {
			atomic.AddUint32(&actualSentMsg, 1)
		} else {
			panic("suppose not pass")
		}
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

		require.True(t, atomic.LoadUint32(&actualSentMsg) < expectedSentMsg)
	}()

	go ea.Listen()
	go eb.Listen()

	require.NoError(t, ea.WritePacket(sentMsg, eb.Addr()))

	time.Sleep(waitPeriod)
}

func testConcurrentWrite(t *testing.T) {
	defer goleak.VerifyNone(t)

	var expected int = 256
	var multipleFactor int = 1000

	cs, es, tc := newEndpointsForConcurrent(t, expected, multipleFactor)

	defer func() {
		tc.wait()

		// Note: Guarantee that all messages are deliverd
		time.Sleep(100 * time.Millisecond)

		require.NoError(t, cs[0].SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, cs[1].SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, cs[2].SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, cs[3].SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, cs[4].SetDeadline(time.Now().Add(1*time.Millisecond)))

		require.NoError(t, es[0].Close())
		require.NoError(t, es[1].Close())
		require.NoError(t, es[2].Close())
		require.NoError(t, es[3].Close())
		require.NoError(t, es[4].Close())

		cs[0].Close()
		cs[1].Close()
		cs[2].Close()
		cs[3].Close()
		cs[4].Close()

		require.EqualValues(t, tc.expected, uniqSort(tc.actual))
	}()

	for i := 0; i < 4; i++ {
		tc.wg.Add(1)
		s := tc.expected[len(tc.expected)*i/4 : len(tc.expected)*(i+1)/4]
		go func(i int) {
			defer tc.done()
			for j := 0; j < len(s); j++ {
				data := bytes.Repeat([]byte("x"), s[j])
				require.NoError(t, es[0].WritePacket(data, es[i+1].Addr()))
			}
		}(i)
	}
}

func testConcurrentRead(t *testing.T) {
	defer goleak.VerifyNone(t)

	var expected int = 256
	var multipleFactor int = 1000

	cs, es, tc := newEndpointsForConcurrent(t, expected, multipleFactor)

	defer func() {
		tc.wait()

		// Note: Guarantee that all messages are deliverd
		time.Sleep(100 * time.Millisecond)

		require.NoError(t, cs[0].SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, cs[1].SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, cs[2].SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, cs[3].SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, cs[4].SetDeadline(time.Now().Add(1*time.Millisecond)))

		require.NoError(t, es[0].Close())
		require.NoError(t, es[1].Close())
		require.NoError(t, es[2].Close())
		require.NoError(t, es[3].Close())
		require.NoError(t, es[4].Close())

		cs[0].Close()
		cs[1].Close()
		cs[2].Close()
		cs[3].Close()
		cs[4].Close()

		require.EqualValues(t, tc.expected, uniqSort(tc.actual))
	}()

	for i := 0; i < 4; i++ {
		tc.wg.Add(1)
		s := tc.expected[len(tc.expected)*i/4 : len(tc.expected)*(i+1)/4]
		go func(i int) {
			defer tc.done()
			for j := 0; j < len(s); j++ {
				data := bytes.Repeat([]byte("x"), s[j])
				require.NoError(t, es[i+1].WritePacket(data, es[0].Addr()))
			}
		}(i)
	}
}

// Note: This struct is test for testConcurrentWrite and TestConcurrentRead
// The purpose for this struct is to prevent race condition of WaitGroup
type testConcurrent struct {
	mu       sync.Mutex
	wg       sync.WaitGroup
	expected []int
	actual   []int
}

func newTestConcurrent(cap, multipleFactor int) *testConcurrent {
	return &testConcurrent{expected: genNumSlice(cap, multipleFactor)}
}

func (t *testConcurrent) done() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.wg.Done()
}

func (t *testConcurrent) wait() {
	t.wg.Wait()
}

func (t *testConcurrent) append(length int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.actual = append(t.actual, length)
}

func newEndpointsForConcurrent(t *testing.T, expected, multipleFactor int) (cs []net.PacketConn, es []*Endpoint, tc *testConcurrent) {
	tc = newTestConcurrent(expected, multipleFactor)

	recvHandler := func(buf []byte) {
		tc.append(len(buf))
	}

	errHandler := func(err error) {
		require.NoError(t, err)
	}

	for i := 0; i < 5; i++ {
		c, e := newEndpoint(t, recvHandler, errHandler)
		cs = append(cs, c)
		es = append(es, e)
		go e.Listen()
	}

	return
}

func genNumSlice(len, multipleFactor int) (s []int) {
	for i := 0; i < len; i++ {
		s = append(s, i*multipleFactor)
	}
	return
}

func uniqSort(s []int) (result []int) {
	sort.Ints(s)
	var pre int
	for i := 0; i < len(s); i++ {
		if i == 0 || s[i] != pre {
			result = append(result, s[i])
		}
		pre = s[i]
	}
	return
}

func BenchmarkWrite(b *testing.B) {
	expected := bytes.Repeat([]byte("x"), int(DefaultFragmentSize)*int(DefaultMaxFragments)/2)

	recvHandler := func(buf []byte) {
		require.EqualValues(b, expected, buf)
	}

	errHandler := func(err error) {
		require.NoError(b, err)
	}

	ca, _ := net.ListenPacket("udp", "127.0.0.1:0")
	ea, _ := NewEndpoint(ca, recvHandler, errHandler, nil)
	cb, _ := net.ListenPacket("udp", "127.0.0.1:0")
	eb, _ := NewEndpoint(cb, recvHandler, errHandler, nil)

	defer func() {
		require.NoError(b, ca.SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(b, cb.SetDeadline(time.Now().Add(1*time.Millisecond)))

		require.NoError(b, ea.Close())
		require.NoError(b, eb.Close())

		ca.Close()
		cb.Close()
	}()

	go ea.Listen()
	go eb.Listen()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		require.NoError(b, ea.WritePacket(expected, eb.Addr()))
	}
}
