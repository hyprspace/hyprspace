package svc

import (
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bytePipe is a mock streamable that provides bidirectional byte channels.
type bytePipe struct {
	mu     sync.Mutex
	rx     chan []byte
	tx     chan []byte
	closed bool
}

func newBytePipe() *bytePipe {
	return &bytePipe{
		rx: make(chan []byte, 16),
		tx: make(chan []byte, 16),
	}
}

func (p *bytePipe) Write(b []byte) (int, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	p.mu.Unlock()
	p.rx <- append([]byte(nil), b...)
	return len(b), nil
}

func (p *bytePipe) Read(b []byte) (int, error) {
	data := <-p.tx
	if data == nil {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
		return 0, io.EOF
	}
	return copy(b, data), nil
}

func (p *bytePipe) send(data []byte) {
	p.tx <- append([]byte(nil), data...)
}

func (p *bytePipe) signalEOF() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.tx <- nil
}

// collectFrom reads exactly n items from pipeB's rx channel, using a done signal for synchronization.
// timeout is a safety net to prevent tests from hanging forever.
func collectFrom(p *bytePipe, n int, timeout time.Duration) ([][]byte, error) {
	var results [][]byte
	done := make(chan struct{})

	go func() {
		defer close(done)
		for len(results) < n {
			data, ok := <-p.rx
			if !ok || data == nil {
				return
			}
			results = append(results, append([]byte(nil), data...))
		}
	}()

	select {
	case <-done:
		return results, nil
	case <-time.After(timeout):
		return results, nil
	}
}

func Test_pipe_singleDirection(t *testing.T) {
	pipeA := newBytePipe()
	pipeB := newBytePipe()

	pipeA.rx = pipeB.tx
	pipeB.rx = pipeA.tx

	go pipe(pipeA, pipeB)

	pipeA.send([]byte("hello"))
	pipeA.send([]byte(" world"))

	dataB, err := collectFrom(pipeB, 2, 500*time.Millisecond)
	require.NoError(t, err)
	require.Len(t, dataB, 2, "pipeB should receive 2 messages")
	assert.Equal(t, []byte("hello"), dataB[0])
	assert.Equal(t, []byte(" world"), dataB[1])
}

func Test_pipe_singleDirection_EOF(t *testing.T) {
	pipeA := newBytePipe()
	pipeB := newBytePipe()

	pipeA.rx = pipeB.tx
	pipeB.rx = pipeA.tx

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pipe(pipeA, pipeB)
	}()

	pipeA.send([]byte("hello"))
	pipeA.signalEOF()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("pipe() did not terminate after EOF")
	}
}

func Test_pipe_bidirectional(t *testing.T) {
	pipeA := newBytePipe()
	pipeB := newBytePipe()

	pipeA.rx = pipeB.tx
	pipeB.rx = pipeA.tx

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pipe(pipeA, pipeB)
	}()

	pipeA.send([]byte("from A"))
	pipeB.send([]byte("from B"))

	// Collect 1 message from pipeB (should be "from A" forwarded from A)
	dataB, err := collectFrom(pipeB, 1, 500*time.Millisecond)
	require.NoError(t, err)
	require.Len(t, dataB, 1, "pipeB should receive 1 message from A")
	assert.Equal(t, []byte("from A"), dataB[0])

	// Collect 1 message from pipeA (should be "from B" forwarded from B)
	dataA, err := collectFrom(pipeA, 1, 500*time.Millisecond)
	require.NoError(t, err)
	require.Len(t, dataA, 1, "pipeA should receive 1 message from B")
	assert.Equal(t, []byte("from B"), dataA[0])
}

func Test_pipe_interleaving(t *testing.T) {
	pipeA := newBytePipe()
	pipeB := newBytePipe()

	pipeA.rx = pipeB.tx
	pipeB.rx = pipeA.tx

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pipe(pipeA, pipeB)
	}()

	// Rapid alternating writes
	pipeA.send([]byte("a"))
	pipeB.send([]byte("b"))
	pipeA.send([]byte("c"))
	pipeB.send([]byte("d"))

	// Collect 2 messages from pipeB (should be "a" and "c" from A)
	dataB, err := collectFrom(pipeB, 2, 500*time.Millisecond)
	require.NoError(t, err)
	require.Len(t, dataB, 2, "pipeB should receive 2 messages")
	assert.Equal(t, []byte("a"), dataB[0])
	assert.Equal(t, []byte("c"), dataB[1])

	// Collect 2 messages from pipeA (should be "b" and "d" from B)
	dataA, err := collectFrom(pipeA, 2, 500*time.Millisecond)
	require.NoError(t, err)
	require.Len(t, dataA, 2, "pipeA should receive 2 messages")
	assert.Equal(t, []byte("b"), dataA[0])
	assert.Equal(t, []byte("d"), dataA[1])
}

func Test_pipe_no_leak_on_block(t *testing.T) {
	pipeA := newBytePipe()
	pipeB := newBytePipe()

	pipeA.rx = pipeB.tx
	pipeB.rx = pipeA.tx

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pipe(pipeA, pipeB)
	}()

	pipeA.send([]byte("start"))
	pipeB.send([]byte("response"))

	time.Sleep(100 * time.Millisecond)

	pipeA.signalEOF()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("pipe goroutine leaked - did not exit after EOF")
	}
}

func Test_toChan_produces_data(t *testing.T) {
	pipe := newBytePipe()
	ch := toChan(pipe)

	pipe.send([]byte("hello"))
	pipe.send([]byte(" world"))
	pipe.signalEOF() // Cleanly terminate the toChan goroutine

	var results [][]byte
	for data := range ch {
		if data == nil {
			break
		}
		results = append(results, append([]byte(nil), data...))
	}

	require.Len(t, results, 2)
	assert.Equal(t, []byte("hello"), results[0])
	assert.Equal(t, []byte(" world"), results[1])
}

func Test_toChan_EOF_closes_channel(t *testing.T) {
	pipe := newBytePipe()

	ch := toChan(pipe)

	pipe.send([]byte("data"))
	pipe.signalEOF()

	var results [][]byte
	for data := range ch {
		if data == nil {
			break
		}
		results = append(results, append([]byte(nil), data...))
	}

	require.Len(t, results, 1)
	assert.Equal(t, []byte("data"), results[0])
}
