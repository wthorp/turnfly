// Package relay implements the experimental relay-pair mode for turnfly.
// Two TURN servers in different Fly regions communicate over a private
// QUIC tunnel, forwarding relayed media packets.
//
// This package is experimental. See SCOPE.md Phase 4 for success criteria.
package relay

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	// FrameMagic is the 4-byte magic number identifying a relay frame.
	FrameMagic = 0x5455524E // "TURN"

	// FrameHeaderSize is the fixed header size in bytes:
	//   4 (magic) + 16 (session_id) + 2 (flow_id) + 1 (direction) +
	//   8 (timestamp) + 2 (payload_len) = 33 bytes
	FrameHeaderSize = 33

	// MaxPayloadSize is the maximum payload size in a single frame.
	// Media packets over QUIC datagrams should stay under the path MTU.
	MaxPayloadSize = 1200
)

// Direction indicates the direction of a relayed packet.
type Direction uint8

const (
	// DirClientToPeer means the packet is from the TURN client toward the peer.
	DirClientToPeer Direction = 0

	// DirPeerToClient means the packet is from the peer toward the TURN client.
	DirPeerToClient Direction = 1
)

func (d Direction) String() string {
	switch d {
	case DirClientToPeer:
		return "client_to_peer"
	case DirPeerToClient:
		return "peer_to_client"
	default:
		return fmt.Sprintf("unknown(%d)", d)
	}
}

// FrameID is a 128-bit session identifier.
type FrameID [16]byte

// Frame represents a relayed media packet with routing metadata.
type Frame struct {
	SessionID FrameID
	FlowID    uint16
	Direction Direction
	Timestamp time.Time
	Payload   []byte
}

var (
	ErrInvalidMagic    = errors.New("invalid frame magic")
	ErrPayloadTooLarge = errors.New("payload exceeds maximum size")
	ErrFrameTooShort   = errors.New("frame too short for header")
)

// Encode serializes a Frame into a binary buffer.
// Returns the encoded bytes or an error if the payload is too large.
func (f *Frame) Encode() ([]byte, error) {
	if len(f.Payload) > MaxPayloadSize {
		return nil, fmt.Errorf("%w: %d > %d", ErrPayloadTooLarge, len(f.Payload), MaxPayloadSize)
	}

	buf := make([]byte, FrameHeaderSize+len(f.Payload))

	// Magic (4 bytes, big-endian).
	binary.BigEndian.PutUint32(buf[0:4], FrameMagic)

	// Session ID (16 bytes).
	copy(buf[4:20], f.SessionID[:])

	// Flow ID (2 bytes, big-endian).
	binary.BigEndian.PutUint16(buf[20:22], f.FlowID)

	// Direction (1 byte).
	buf[22] = byte(f.Direction)

	// Timestamp (8 bytes, unix microseconds, big-endian).
	binary.BigEndian.PutUint64(buf[23:31], uint64(f.Timestamp.UnixMicro()))

	// Payload length (2 bytes, big-endian).
	binary.BigEndian.PutUint16(buf[31:33], uint16(len(f.Payload)))

	// Payload.
	copy(buf[33:], f.Payload)

	return buf, nil
}

// DecodeFrame decodes a binary buffer into a Frame.
// Returns the decoded frame or an error if the buffer is malformed.
func DecodeFrame(data []byte) (*Frame, error) {
	if len(data) < FrameHeaderSize {
		return nil, fmt.Errorf("%w: got %d bytes, need %d", ErrFrameTooShort, len(data), FrameHeaderSize)
	}

	// Check magic.
	magic := binary.BigEndian.Uint32(data[0:4])
	if magic != FrameMagic {
		return nil, fmt.Errorf("%w: got 0x%08X, expected 0x%08X", ErrInvalidMagic, magic, FrameMagic)
	}

	f := &Frame{}

	// Session ID.
	copy(f.SessionID[:], data[4:20])

	// Flow ID.
	f.FlowID = binary.BigEndian.Uint16(data[20:22])

	// Direction.
	f.Direction = Direction(data[22])

	// Timestamp.
	ts := binary.BigEndian.Uint64(data[23:31])
	f.Timestamp = time.UnixMicro(int64(ts))

	// Payload length.
	payloadLen := binary.BigEndian.Uint16(data[31:33])

	if int(payloadLen) > len(data)-FrameHeaderSize {
		return nil, fmt.Errorf("payload length %d exceeds remaining data %d", payloadLen, len(data)-FrameHeaderSize)
	}

	// Payload.
	f.Payload = make([]byte, payloadLen)
	copy(f.Payload, data[FrameHeaderSize:FrameHeaderSize+int(payloadLen)])

	return f, nil
}

// NewFrameID generates a new random 128-bit session ID from a UUID-like
// source. For production use, this should use crypto/rand.
func NewFrameID(data []byte) FrameID {
	var id FrameID
	copy(id[:], data[:16])
	return id
}
