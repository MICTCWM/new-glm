package common

import "testing"

func TestLimitedCaptureBufferBoundsWrites(t *testing.T) {
	buf := NewLimitedCaptureBuffer(4)
	if n, err := buf.Write([]byte("abcdef")); err != nil || n != 6 {
		t.Fatalf("Write() = (%d, %v), want (6, nil)", n, err)
	}
	if got := string(buf.Bytes()); got != "abcd" {
		t.Fatalf("captured body = %q, want %q", got, "abcd")
	}
	if !buf.Truncated() {
		t.Fatal("buffer should report truncation")
	}
}

func TestLimitCaptureBytesZeroDisablesCapture(t *testing.T) {
	if got := LimitCaptureBytes([]byte("body"), 0); got != nil {
		t.Fatalf("LimitCaptureBytes() = %q, want nil", got)
	}
}
