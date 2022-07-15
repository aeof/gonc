package main

import (
	"net"
	"time"
)

// timeoutConn is a wrapper type for net.Conn, which sets deadline for every read/write.
// If readTimeout is zero, no read deadline is set for the connection, otherwise a read
// deadline is set. The rule works for writeTimeout, too.
type timeoutConn struct {
	// the actual network connection
	net.Conn

	// we need this field to remember if a readDeadline is specifically set
	isReadDeadlineSet bool

	// we need this field to remember if a writeDeadline is specifically set
	isWriteDeadlineSet bool

	// timeout for every read from the connection
	ReadTimeout time.Duration

	// timeout for every write to the connection
	WriteTimeout time.Duration
}

func NewTimeoutConn(conn net.Conn, readTimeout, writeTimeout time.Duration) net.Conn {
	return &timeoutConn{
		Conn:         conn,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
}

// Every read from the timeoutConn may timeout.
// If readDeadline is set, the deadline is readDeadline.
//
// If readTimeout is set, the deadline is time.Now().After(readTimeout).
//
// If readTimeout is not set, read will not timeout.
func (tr *timeoutConn) Read(p []byte) (n int, err error) {
	if !tr.isReadDeadlineSet && tr.ReadTimeout != 0 {
		tr.Conn.SetReadDeadline(time.Now().Add(tr.ReadTimeout))
	}
	return tr.Conn.Read(p)
}

// Every write to the timeoutConn may timeout.
// If writeDeadline is set, the deadline is writeDeadline.
//
// If writeTimeout is set, the deadline is time.Now().After(writeTimeout).
//
// If writeTimeout is not set, write will not timeout.
func (tr *timeoutConn) Write(p []byte) (n int, err error) {
	if !tr.isWriteDeadlineSet && tr.WriteTimeout != 0 {
		tr.SetWriteDeadline(time.Now().Add(tr.WriteTimeout))
	}
	return tr.Conn.Write(p)
}

func (tr *timeoutConn) SetDeadline(t time.Time) error {
	if err := tr.SetReadDeadline(t); err != nil {
		return err
	}
	if err := tr.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

func (tr *timeoutConn) SetReadDeadline(t time.Time) error {
	var zero time.Time
	if t == zero {
		tr.isReadDeadlineSet = false
	} else {
		tr.isReadDeadlineSet = true
	}
	return tr.Conn.SetReadDeadline(t)
}

func (tr *timeoutConn) SetWriteDeadline(t time.Time) error {
	var zero time.Time
	if t == zero {
		tr.isWriteDeadlineSet = false
	} else {
		tr.isWriteDeadlineSet = true
	}
	return tr.Conn.SetWriteDeadline(t)
}
