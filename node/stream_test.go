package node

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/hyprspace/hyprspace/internal/protocol"
)

func TestReadStreamPacketReadsValidPacket(t *testing.T) {
	var packet [protocol.MaxPacketSize]byte
	payload := []byte{0xde, 0xad, 0xbe, 0xef}
	var input bytes.Buffer

	err := binary.Write(&input, binary.LittleEndian, uint16(len(payload)))
	if err != nil {
		t.Fatalf("failed to write packet size: %v", err)
	}
	input.Write(payload)

	size, err := readStreamPacket(&input, packet[:])
	if err != nil {
		t.Fatalf("expected valid packet, got error: %v", err)
	}
	if int(size) != len(payload) {
		t.Fatalf("expected size %d, got %d", len(payload), size)
	}
	if !bytes.Equal(packet[:size], payload) {
		t.Fatalf("expected payload %x, got %x", payload, packet[:size])
	}
}

func TestReadStreamPacketRejectsOversizedPacket(t *testing.T) {
	var packet [protocol.MaxPacketSize]byte
	var input bytes.Buffer

	err := binary.Write(&input, binary.LittleEndian, uint16(len(packet)+1))
	if err != nil {
		t.Fatalf("failed to write packet size: %v", err)
	}

	_, err = readStreamPacket(&input, packet[:])
	if !errors.Is(err, errInvalidPacketSize) {
		t.Fatalf("expected errInvalidPacketSize, got %v", err)
	}
}

func TestReadStreamPacketRequiresFullSizeHeader(t *testing.T) {
	var packet [protocol.MaxPacketSize]byte
	input := bytes.NewBuffer([]byte{0x01})

	_, err := readStreamPacket(input, packet[:])
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
	}
}

func TestReadStreamPacketRejectsZeroSize(t *testing.T) {
	packet := make([]byte, protocol.MaxPacketSize)
	input := bytes.NewReader([]byte{0x00, 0x00})

	_, err := readStreamPacket(input, packet)
	if !errors.Is(err, errInvalidPacketSize) {
		t.Fatalf("expected errInvalidPacketSize, got %v", err)
	}
}

func TestReadStreamPacketAcceptsMaximumSize(t *testing.T) {
	packet := make([]byte, protocol.MaxPacketSize)
	payload := make([]byte, protocol.MaxPacketSize)

	var input bytes.Buffer
	if err := binary.Write(
		&input,
		binary.LittleEndian,
		uint16(len(payload)),
	); err != nil {
		t.Fatalf("write packet size: %v", err)
	}

	input.Write(payload)

	size, err := readStreamPacket(&input, packet)
	if err != nil {
		t.Fatalf("read maximum-size packet: %v", err)
	}

	if int(size) != len(payload) {
		t.Fatalf("expected size %d, got %d", len(payload), size)
	}
}

func TestReadStreamPacketRequiresFullPayload(t *testing.T) {
	packet := make([]byte, protocol.MaxPacketSize)

	var input bytes.Buffer
	if err := binary.Write(
		&input,
		binary.LittleEndian,
		uint16(4),
	); err != nil {
		t.Fatalf("write packet size: %v", err)
	}

	input.Write([]byte{0xde, 0xad})

	_, err := readStreamPacket(&input, packet)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
	}
}

func TestReadStreamPacketReturnsEOFForEmptyStream(t *testing.T) {
	packet := make([]byte, protocol.MaxPacketSize)

	_, err := readStreamPacket(bytes.NewReader(nil), packet)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}
