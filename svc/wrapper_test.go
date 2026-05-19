package svc

import (
	"io"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStream struct {
	readData   []byte
	readIdx    int
	writeData  [][]byte
	closeCalls int
	deadlineErr error
	readErr    error
	writeErr   error
}

func (m *mockStream) Read(b []byte) (int, error) {
	if m.readErr != nil {
		return 0, m.readErr
	}
	if m.readIdx >= len(m.readData) {
		return 0, io.EOF
	}
	n := copy(b, m.readData[m.readIdx:])
	m.readIdx += n
	return n, nil
}

func (m *mockStream) Write(b []byte) (int, error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	m.writeData = append(m.writeData, append([]byte(nil), b...))
	return len(b), nil
}

func (m *mockStream) Close() error {
	m.closeCalls++
	return nil
}

func (m *mockStream) CloseRead() error  { return nil }
func (m *mockStream) CloseWrite() error { return nil }
func (m *mockStream) Reset() error            { return nil }
func (m *mockStream) ResetWithError(network.StreamErrorCode) error { return nil }
func (m *mockStream) SetDeadline(t time.Time) error { return m.deadlineErr }
func (m *mockStream) SetReadDeadline(t time.Time) error { return m.deadlineErr }
func (m *mockStream) SetWriteDeadline(t time.Time) error { return m.deadlineErr }
func (m *mockStream) ID() string               { return "0" }
func (m *mockStream) Protocol() protocol.ID    { return "" }
func (m *mockStream) SetProtocol(protocol.ID) error { return nil }
func (m *mockStream) Stat() network.Stats      { return network.Stats{} }
func (m *mockStream) Conn() network.Conn       { return nil }
func (m *mockStream) Scope() network.StreamScope { return nil }

func Test_StreamWrapper_Read(t *testing.T) {
	mock := &mockStream{readData: []byte("hello world")}
	sw := StreamWrapper{Stream: mock}

	buf := make([]byte, 1024)
	n, err := sw.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 11, n)
	assert.Equal(t, "hello world", string(buf[:n]))
}

func Test_StreamWrapper_Read_EOF(t *testing.T) {
	mock := &mockStream{readData: []byte("hi")}
	sw := StreamWrapper{Stream: mock}

	buf := make([]byte, 1024)
	n, err := sw.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, "hi", string(buf[:n]))

	n, err = sw.Read(buf)
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, io.EOF)
}

func Test_StreamWrapper_Write(t *testing.T) {
	mock := &mockStream{}
	sw := StreamWrapper{Stream: mock}

	n, err := sw.Write([]byte("test data"))
	require.NoError(t, err)
	assert.Equal(t, 9, n)
	require.Len(t, mock.writeData, 1)
	assert.Equal(t, []byte("test data"), mock.writeData[0])
}

func Test_StreamWrapper_Write_Multiple(t *testing.T) {
	mock := &mockStream{}
	sw := StreamWrapper{Stream: mock}

	sw.Write([]byte("a"))
	sw.Write([]byte("b"))

	require.Len(t, mock.writeData, 2)
	assert.Equal(t, []byte("a"), mock.writeData[0])
	assert.Equal(t, []byte("b"), mock.writeData[1])
}

func Test_StreamWrapper_Read_Error(t *testing.T) {
	mock := &mockStream{readErr: io.ErrUnexpectedEOF}
	sw := StreamWrapper{Stream: mock}

	_, err := sw.Read(make([]byte, 10))
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func Test_StreamWrapper_Write_Error(t *testing.T) {
	mock := &mockStream{writeErr: io.ErrClosedPipe}
	sw := StreamWrapper{Stream: mock}

	_, err := sw.Write([]byte("data"))
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

func Test_StreamWrapper_Close(t *testing.T) {
	mock := &mockStream{}
	sw := StreamWrapper{Stream: mock}

	sw.Close()
	assert.Equal(t, 1, mock.closeCalls)

	sw.Close()
	assert.Equal(t, 2, mock.closeCalls)
}

func Test_StreamWrapper_Deadlines(t *testing.T) {
	mock := &mockStream{deadlineErr: io.ErrClosedPipe}
	sw := StreamWrapper{Stream: mock}

	assert.ErrorIs(t, sw.SetDeadline(time.Now()), io.ErrClosedPipe)
	assert.ErrorIs(t, sw.SetReadDeadline(time.Now()), io.ErrClosedPipe)
	assert.ErrorIs(t, sw.SetWriteDeadline(time.Now()), io.ErrClosedPipe)
}

func Test_StreamWrapper_WrapStream(t *testing.T) {
	mock := &mockStream{}
	sw := StreamWrapper{Stream: mock}

	assert.Equal(t, mock, sw.Stream)
}
