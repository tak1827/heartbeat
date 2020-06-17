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

func TestConcurrentWrite(t *testing.T) {
	defer goleak.VerifyNone(t)

	var expected int = 256
	var multipleFactor int = 1000
	tc := newTestConcurrentWrite(expected, multipleFactor)

	recvHandler := func(buf []byte) {
		tc.append(len(buf))
	}

	errHandler := func(err error) {
		require.NoError(t, err)
	}

	ca, ea := newEndpoint(t, recvHandler, errHandler)
	cb, eb := newEndpoint(t, recvHandler, errHandler)
	cc, ec := newEndpoint(t, recvHandler, errHandler)
	cd, ed := newEndpoint(t, recvHandler, errHandler)
	ce, ee := newEndpoint(t, recvHandler, errHandler)

	defer func() {
		tc.wait()

		// Note: Guarantee that all messages are deliverd
		time.Sleep(100 * time.Millisecond)

		require.NoError(t, ca.SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, cb.SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, cc.SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, cd.SetDeadline(time.Now().Add(1*time.Millisecond)))
		require.NoError(t, ce.SetDeadline(time.Now().Add(1*time.Millisecond)))

		require.NoError(t, ea.Close())
		require.NoError(t, eb.Close())
		require.NoError(t, ec.Close())
		require.NoError(t, ed.Close())
		require.NoError(t, ee.Close())

		ca.Close()
		cb.Close()
		cc.Close()
		cd.Close()
		ce.Close()

		require.EqualValues(t, tc.expected, uniqSort(tc.actual))
	}()

	go ea.Listen()
	go eb.Listen()
	go ec.Listen()
	go ed.Listen()
	go ee.Listen()

	tc.wg.Add(1)
	sB := tc.expected[0 : len(tc.expected)/4]
	go func() {
		defer tc.done()
		for i := 0; i < len(sB); i++ {
			data := bytes.Repeat([]byte("x"), sB[i])
			require.NoError(t, ea.WritePacket(data, eb.Addr()))
		}
	}()

	tc.wg.Add(1)
	sC := tc.expected[len(tc.expected)/4 : len(tc.expected)*2/4]
	go func() {
		defer tc.done()
		for i := 0; i < len(sC); i++ {
			data := bytes.Repeat([]byte("x"), sC[i])
			require.NoError(t, ea.WritePacket(data, ec.Addr()))
		}
	}()

	tc.wg.Add(1)
	sD := tc.expected[len(tc.expected)*2/4 : len(tc.expected)*3/4]
	go func() {
		defer tc.done()
		for i := 0; i < len(sD); i++ {
			data := bytes.Repeat([]byte("x"), sD[i])
			require.NoError(t, ea.WritePacket(data, ed.Addr()))
		}
	}()

	tc.wg.Add(1)
	sE := tc.expected[len(tc.expected)*3/4:]
	go func() {
		defer tc.done()
		for i := 0; i < len(sE); i++ {
			data := bytes.Repeat([]byte("x"), sE[i])
			require.NoError(t, ea.WritePacket(data, ee.Addr()))
		}
	}()
}

// Note: This struct is test for TestConcurrentWrite
// The purpose for this struct is to prevent race condition of WaitGroup
type testConcurrentWrite struct {
	mu       sync.Mutex
	wg       sync.WaitGroup
	expected []int
	actual   []int
}

func newTestConcurrentWrite(cap, multipleFactor int) *testConcurrentWrite {
	return &testConcurrentWrite{expected: genNumSlice(cap, multipleFactor)}
}

func (t *testConcurrentWrite) done() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.wg.Done()
}

func (t *testConcurrentWrite) wait() {
	t.wg.Wait()
}

func (t *testConcurrentWrite) append(length int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// num, _ := strconv.Atoi(string(buf))
	t.actual = append(t.actual, length)
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
