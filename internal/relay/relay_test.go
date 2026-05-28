package relay

import (
	"testing"
	"time"
)

func TestFrameEncodeDecode(t *testing.T) {
	id, err := NewSessionID()
	if err != nil {
		t.Fatalf("NewSessionID: %v", err)
	}

	payload := []byte("hello relay world")
	f := &Frame{
		SessionID: id,
		FlowID:    42,
		Direction: DirClientToPeer,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if len(data) != FrameHeaderSize+len(payload) {
		t.Errorf("expected %d bytes, got %d", FrameHeaderSize+len(payload), len(data))
	}

	decoded, err := DecodeFrame(data)
	if err != nil {
		t.Fatalf("DecodeFrame() error = %v", err)
	}

	if decoded.SessionID != f.SessionID {
		t.Errorf("SessionID mismatch: %x != %x", decoded.SessionID, f.SessionID)
	}
	if decoded.FlowID != f.FlowID {
		t.Errorf("FlowID mismatch: %d != %d", decoded.FlowID, f.FlowID)
	}
	if decoded.Direction != f.Direction {
		t.Errorf("Direction mismatch: %d != %d", decoded.Direction, f.Direction)
	}
	if string(decoded.Payload) != string(payload) {
		t.Errorf("Payload mismatch: %q != %q", string(decoded.Payload), string(payload))
	}
}

func TestFrameEncodePayloadTooLarge(t *testing.T) {
	id, _ := NewSessionID()
	f := &Frame{
		SessionID: id,
		Payload:   make([]byte, MaxPayloadSize+1),
	}

	_, err := f.Encode()
	if err == nil {
		t.Fatal("expected error for oversized payload")
	}
}

func TestDecodeFrameInvalidMagic(t *testing.T) {
	data := make([]byte, FrameHeaderSize)
	// Wrong magic.
	data[0] = 0xDE
	data[1] = 0xAD
	data[2] = 0xBE
	data[3] = 0xEF

	_, err := DecodeFrame(data)
	if err == nil {
		t.Fatal("expected error for invalid magic")
	}
}

func TestDecodeFrameTooShort(t *testing.T) {
	data := make([]byte, 10)
	_, err := DecodeFrame(data)
	if err == nil {
		t.Fatal("expected error for short frame")
	}
}

func TestFrameDirectionRoundTrip(t *testing.T) {
	id, _ := NewSessionID()
	for _, dir := range []Direction{DirClientToPeer, DirPeerToClient} {
		f := &Frame{
			SessionID: id,
			Direction: dir,
			Payload:   []byte("test"),
		}
		data, err := f.Encode()
		if err != nil {
			t.Fatalf("Encode(%v): %v", dir, err)
		}
		decoded, err := DecodeFrame(data)
		if err != nil {
			t.Fatalf("DecodeFrame(%v): %v", dir, err)
		}
		if decoded.Direction != dir {
			t.Errorf("expected direction %d, got %d", dir, decoded.Direction)
		}
	}
}

func TestFrameZeroPayload(t *testing.T) {
	id, _ := NewSessionID()
	f := &Frame{
		SessionID: id,
		Payload:   []byte{},
	}
	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(data) != FrameHeaderSize {
		t.Errorf("expected %d bytes for empty payload, got %d", FrameHeaderSize, len(data))
	}

	decoded, err := DecodeFrame(data)
	if err != nil {
		t.Fatalf("DecodeFrame() error = %v", err)
	}
	if len(decoded.Payload) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(decoded.Payload))
	}
}

func TestNewSessionID(t *testing.T) {
	id1, err := NewSessionID()
	if err != nil {
		t.Fatalf("NewSessionID: %v", err)
	}
	id2, err := NewSessionID()
	if err != nil {
		t.Fatalf("NewSessionID: %v", err)
	}
	if id1 == id2 {
		t.Error("expected unique session IDs")
	}
}

func TestSessionLifecycle(t *testing.T) {
	mgr := NewManager(5 * time.Second)

	id, _ := NewSessionID()
	s := mgr.Create(id, "peer1:4443")

	if s.State != StateActive {
		t.Errorf("expected active, got %s", s.State)
	}

	got, ok := mgr.Get(id)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if got.PeerAddr != "peer1:4443" {
		t.Errorf("expected peer1:4443, got %s", got.PeerAddr)
	}

	// Record some traffic.
	s.RecordPacketIn(100)
	s.RecordPacketOut(200)

	if s.Stats.PacketsIn != 1 || s.Stats.BytesIn != 100 {
		t.Errorf("expected 1 packet in / 100 bytes in, got %d/%d", s.Stats.PacketsIn, s.Stats.BytesIn)
	}

	// Close.
	if !mgr.Close(id) {
		t.Error("expected Close to succeed")
	}

	got, ok = mgr.Get(id)
	if !ok {
		t.Fatal("expected session to still exist after close")
	}
	if got.State != StateClosed {
		t.Errorf("expected closed, got %s", got.State)
	}
}

func TestSessionGC(t *testing.T) {
	mgr := NewManager(1 * time.Millisecond) // very short timeout

	id, _ := NewSessionID()
	mgr.Create(id, "peer1:4443")

	// Wait for GC.
	time.Sleep(10 * time.Millisecond)

	removed := mgr.GC()
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	if mgr.Count() != 0 {
		t.Errorf("expected 0 sessions after GC, got %d", mgr.Count())
	}
}

func TestSessionRecordDrop(t *testing.T) {
	mgr := NewManager(60 * time.Second)
	id, _ := NewSessionID()
	s := mgr.Create(id, "peer1:4443")

	s.RecordDrop()
	if s.Stats.PacketsDropped != 1 {
		t.Errorf("expected 1 drop, got %d", s.Stats.PacketsDropped)
	}
}

func TestSessionDelete(t *testing.T) {
	mgr := NewManager(60 * time.Second)
	id, _ := NewSessionID()
	mgr.Create(id, "peer1:4443")

	mgr.Delete(id)
	if mgr.Count() != 0 {
		t.Errorf("expected 0 after delete, got %d", mgr.Count())
	}
}

func TestSessionList(t *testing.T) {
	mgr := NewManager(60 * time.Second)

	id1, _ := NewSessionID()
	id2, _ := NewSessionID()

	mgr.Create(id1, "peer1:4443")
	mgr.Create(id2, "peer2:4443")

	list := mgr.List()
	if len(list) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(list))
	}
}

func TestGenerateSelfSignedCert(t *testing.T) {
	cert, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Error("expected non-empty certificate chain")
	}
	if cert.PrivateKey == nil {
		t.Error("expected non-nil private key")
	}
}
