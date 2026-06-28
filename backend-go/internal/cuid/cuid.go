// Package cuid generates collision-resistant ids shaped like Prisma's cuid()
// default (a leading 'c' followed by 24 base36 chars). New rows created by the
// Go backend use these so ids look like the ones Prisma produced.
package cuid

import (
	"crypto/rand"
	"encoding/binary"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

const blockSize = 4
const base = 36

var counter uint64

var fingerprint = computeFingerprint()

func toBase36(n uint64) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	var b []byte
	for n > 0 {
		b = append([]byte{digits[n%base]}, b...)
		n /= base
	}
	return string(b)
}

func pad(s string, size int) string {
	if len(s) >= size {
		return s[len(s)-size:]
	}
	return strings.Repeat("0", size-len(s)) + s
}

func computeFingerprint() string {
	pid := os.Getpid()
	host, _ := os.Hostname()
	hostSum := len(host) + base
	for _, c := range host {
		hostSum += int(c)
	}
	return pad(toBase36(uint64(pid)), 2) + pad(toBase36(uint64(hostSum)), 2)
}

func randomBlock() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand failure is non-recoverable in practice.
		return "0000"
	}
	return pad(toBase36(uint64(binary.BigEndian.Uint32(buf[:]))), blockSize)
}

// New returns a new cuid-shaped id, e.g. "c<timestamp><counter><fingerprint><random>".
func New() string {
	ts := toBase36(uint64(time.Now().UnixMilli()))
	count := pad(toBase36(atomic.AddUint64(&counter, 1)), blockSize)
	return "c" + ts + count + fingerprint + randomBlock() + randomBlock()
}
