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
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return 0, io.EOF
	}
	p.mu.Unlock()
	data := <-p.tx
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	n := copy(b, data)
	return n, io.EOF
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

// drainReader reads all available data from a pipe into a result slice.
// It stops when it reads nil (EOF) or times out.
func drainReader(p *bytePipe, timeout time.Duration) [][]byte {
	var results [][]byte
	done := make(chan struct{})

	go func() {
		for {
			p.mu.Lock()
			closed := p.closed
			p.mu.Unlock()
			if closed {
				break
			}
			select {
			case data := <-p.rx:
				if data == nil {
					return
				}
				results = append(results, append([]byte(nil), data...))
			case <-time.After(timeout):
				return
			}
		}
		done <- struct{}{}
	}()

	select {
	case <-done:
		return results
	case <-time.After(timeout):
		return results
	}
}

func TestT6_pipe_singleDirection(t *testing.T) {
	pipeA := newBytePipe()
	pipeB := newBytePipe()

	pipeA.rx = pipeB.tx
	pipeB.rx = pipeA.tx

	go pipe(pipeA, pipeB)

	pipeA.send([]byte("hello"))
	pipeA.send([]byte(" world"))

	dataB := drainReader(pipeB, 200*time.Millisecond)

	require.Len(t, dataB, 2, "pipeB should receive 2 messages")
	assert.Equal(t, []byte("hello"), dataB[0])
	assert.Equal(t, []byte(" world"), dataB[1])
}

func TestT6_pipe_singleDirection_EOF(t *testing.T) {
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

func TestT7_pipe_bidirectional(t *testing.T) {
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

	time.Sleep(50 * time.Millisecond)

	_ = drainReader(pipeB, 200*time.Millisecond)
	dataA := drainReader(pipeA, 200*time.Millisecond)

	require.Len(t, dataA, 1, "pipeA should receive 1 message from B")
	assert.Equal(t, []byte("from B"), dataA[0])
}

func TestT7_pipe_interleaving(t *testing.T) {
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

	pipeA.send([]byte("a"))
	pipeB.send([]byte("b"))
	pipeA.send([]byte("c"))
	pipeB.send([]byte("d"))

	time.Sleep(50 * time.Millisecond)

	dataB := drainReader(pipeB, 200*time.Millisecond)
	dataA := drainReader(pipeA, 200*time.Millisecond)

	require.Len(t, dataB, 2, "pipeB should receive 2 messages")
	assert.Equal(t, []byte("a"), dataB[0])
	assert.Equal(t, []byte("c"), dataB[1])

	require.Len(t, dataA, 2, "pipeA should receive 2 messages")
	assert.Equal(t, []byte("b"), dataA[0])
	assert.Equal(t, []byte("d"), dataA[1])
}

func TestT7_pipe_no_leak_on_block(t *testing.T) {
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

func TestT8_toChan_produces_data(t *testing.T) {
	pipe := newBytePipe()

	ch := toChan(pipe)

	pipe.send([]byte("hello"))
	pipe.send([]byte(" world"))

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

func TestT8_toChan_EOF_closes_channel(t *testing.T) {
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
