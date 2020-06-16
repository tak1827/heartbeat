package heartbeat

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"net"
	"sort"
	"strconv"
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

func TestHertbeat(t *testing.T) {
	defer goleak.VerifyNone(t)

	waitPeriod := 3 * time.Second

	sentMsg := []byte("data")
	actualSentMsg := uint64(0)
	expectedSentMsg := uint64(11)

	recvHandler := func(buf []byte) {
		if bytes.Compare(buf, sentMsg) == 0 {
			atomic.AddUint64(&actualSentMsg, 1)
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

		require.True(t, actualSentMsg < expectedSentMsg)
	}()

	go ea.Listen()
	go eb.Listen()

	require.NoError(t, ea.WritePacket(sentMsg, eb.Addr()))

	time.Sleep(waitPeriod)
}

func TestConcurrentWrite(t *testing.T) {
	defer goleak.VerifyNone(t)

	var expected int = 1024
	tc := newTestConcurrentWrite(expected)

	recvHandler := func(buf []byte) {
		tc.append(buf)
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
			data := []byte(strconv.Itoa(sB[i]))
			require.NoError(t, ea.WritePacket(data, eb.Addr()))
		}
	}()

	tc.wg.Add(1)
	sC := tc.expected[len(tc.expected)/4 : len(tc.expected)*2/4]
	go func() {
		defer tc.done()
		for i := 0; i < len(sC); i++ {
			data := []byte(strconv.Itoa(sC[i]))
			require.NoError(t, ea.WritePacket(data, ec.Addr()))
		}
	}()

	tc.wg.Add(1)
	sD := tc.expected[len(tc.expected)*2/4 : len(tc.expected)*3/4]
	go func() {
		defer tc.done()
		for i := 0; i < len(sD); i++ {
			data := []byte(strconv.Itoa(sD[i]))
			require.NoError(t, ea.WritePacket(data, ed.Addr()))
		}
	}()

	tc.wg.Add(1)
	sE := tc.expected[len(tc.expected)*3/4:]
	go func() {
		defer tc.done()
		for i := 0; i < len(sE); i++ {
			data := []byte(strconv.Itoa(sE[i]))
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

func newTestConcurrentWrite(cap int) *testConcurrentWrite {
	return &testConcurrentWrite{expected: genNumSlice(cap)}
}

func (t *testConcurrentWrite) done() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.wg.Done()
}

func (t *testConcurrentWrite) wait() {
	t.wg.Wait()
}

func (t *testConcurrentWrite) append(buf []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	num, _ := strconv.Atoi(string(buf))
	t.actual = append(t.actual, num)
}

func genNumSlice(len int) (s []int) {
	for i := 0; i < len; i++ {
		s = append(s, i*10)
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
