package main

import (
	"testing"
)

func Test_findBytePattern(t *testing.T) {
	buf := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}

	pos, err := findBytePattern("09", buf)
	if err != nil {
		t.Errorf("Couldn't find byte pattern: %s", err.Error())
	}
	if pos != int64(buf[pos]) {
		t.Errorf("Couldn't find byte at pos %d", pos)
	}
}
