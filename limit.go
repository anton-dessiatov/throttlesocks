package main

import (
	"io"
	"net"
	"time"

	"golang.org/x/time/rate"
)

// LimitedConnection is a wrapper around net.Conn that limits the rate of its
// Read and Write operations based on given rate.
type LimitedConnection struct {
	inner net.Conn

	limiter        *rate.Limiter
	readNotBefore  time.Time
	writeNotBefore time.Time

	readDeadline  time.Time
	writeDeadline time.Time
	close         chan struct{}
}

// NewLimitedConnection creates a LimitedConnection from net.Conn and a bytes-per-second value
func NewLimitedConnection(inner net.Conn, limiter *rate.Limiter) *LimitedConnection {
	bufSize := limiter.Burst()
	if bufSize > MaxBurstSize {
		bufSize = MaxBurstSize
	}
	return &LimitedConnection{
		inner:   inner,
		limiter: limiter,
		close:   make(chan struct{}),
	}
}

// LocalAddr is an implementation of net.Conn.LocalAddr
func (c *LimitedConnection) LocalAddr() net.Addr {
	return c.inner.LocalAddr()
}

// RemoteAddr is an implementation of net.Conn.RemoteAddr
func (c *LimitedConnection) RemoteAddr() net.Addr {
	return c.inner.RemoteAddr()
}

// Read is an implementation of net.Conn.Read
func (c *LimitedConnection) Read(b []byte) (read int, err error) {
	return c.rateLimitLoop(&c.readNotBefore, &c.readDeadline, c.inner.Read, b)
}

// Write is an implementation of net.Conn.Write
func (c *LimitedConnection) Write(b []byte) (written int, err error) {
	return c.rateLimitLoop(&c.writeNotBefore, &c.writeDeadline, c.inner.Write, b)
}

// The idea is that we read in chunks equal to max burst allowed by multilimiter
// After reading we attempt to reserve time slot for a read chunk. If we succeed
// we go on. If not, we check what happens before - operation deadline or wait
// time. If that's wait time then simply wait and repeat. If it's a deadline
// then set 'not before' timestamp and wait for it upon next invocation.
func (c *LimitedConnection) rateLimitLoop(notBefore *time.Time,
	deadline *time.Time, innerAct func([]byte) (int, error),
	b []byte) (cntr int, err error) {
	if len(b) == 0 {
		return innerAct(b)
	}

	now := time.Now()
	var until time.Time

	if now.Before(*notBefore) {
		until = *notBefore
		if !deadline.IsZero() && deadline.Before(until) {
			until = *deadline
		}
	}

	if !until.IsZero() {
		if c.waitUntil(until) {
			err = io.ErrClosedPipe
			return
		}
	}

	burst := c.limiter.Burst()
	var n int
	if burst > len(b)-cntr {
		burst = len(b) - cntr
	}
	n, err = innerAct(b[cntr:][:burst])
	if n == 0 {
		return
	}

	cntr += n
	until = time.Time{}

	now = time.Now()
	r := c.limiter.ReserveN(now, n)
	act := now.Add(r.DelayFrom(now))
	if now.Before(act) {
		if !deadline.IsZero() && deadline.Before(act) {
			*notBefore = act
			err = timeoutError{}
			return
		}
		until = act
	}
	if !until.IsZero() {
		if c.waitUntil(act) {
			err = io.ErrClosedPipe
			return
		}
	}
	return
}

type timeoutError struct{}

func (timeoutError) Error() string { return "deadline exceeded" }

func (timeoutError) Timeout() bool { return true }

func (timeoutError) Temporary() bool { return true }

// SetDeadline is an implementation of net.Conn.SetDeadline
func (c *LimitedConnection) SetDeadline(t time.Time) error {
	err := c.SetReadDeadline(t)
	if err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

// SetReadDeadline is an implementation of net.Conn.SetReadDeadline
func (c *LimitedConnection) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return c.inner.SetReadDeadline(t)
}

// SetWriteDeadline is an implementation of net.Conn.SetWriteDeadline
func (c *LimitedConnection) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return c.inner.SetWriteDeadline(t)
}

// Close is an implementation of net.Conn.Close
func (c *LimitedConnection) Close() error {
	close(c.close)
	res := c.inner.Close()
	return res
}

// Waits until given time or until connection is closed. Returns
// true if connection was closed and false if time has elapsed
// or if wait was aborted by closing or sending on 'abortWait'
func (c *LimitedConnection) waitUntil(t time.Time) bool {
	timer := time.NewTimer(t.Sub(time.Now()))
	defer timer.Stop()
	select {
	case <-timer.C:
		return false
	case <-c.close:
		return true
	}
}

// MinBurstSize defines minimum size for an limiter burst
const MinBurstSize = 1

// MaxBurstSize defines maximum size for a limiter burst
const MaxBurstSize = 64 * 1024

// NewLimiter creates rate.Limiter for a given bandwidth limit
func NewLimiter(limit rate.Limit) *rate.Limiter {
	return rate.NewLimiter(limit, GetGoodBurst(limit))
}

// GetGoodBurst returns burst size that allows to precisely limit rate
// Returned burst size is no bigger than MaxBurstSize and no less than
// MinBurstSize
func GetGoodBurst(l rate.Limit) int {
	if l == rate.Limit(0) {
		return MaxBurstSize
	}
	// We aim for 20 bursts per second to get good precision. Decrease this
	// value to get better performance, but less precision.
	burstSize := int64(l) / 20
	if burstSize < MinBurstSize {
		burstSize = MinBurstSize
	} else if burstSize > MaxBurstSize {
		burstSize = MaxBurstSize
	}
	return int(burstSize)
}
