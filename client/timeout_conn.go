package client

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

	// timeout for every read from the connection
	ReadTimeout time.Duration

	// timeout for every write to the connection
	WriteTimeout time.Duration
}

// NewTimeoutConn creates a new timeout connection.
func NewTimeoutConn(conn net.Conn, readTimeout, writeTimeout time.Duration) net.Conn {
	return &timeoutConn{
		Conn:         conn,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
}

// Every we read from a timeout connection, we advance its read deadline.
func (tr *timeoutConn) Read(p []byte) (n int, err error) {
	var zero time.Duration
	if tr.ReadTimeout != zero {
		tr.Conn.SetReadDeadline(time.Now().Add(tr.ReadTimeout))
	}
	return tr.Conn.Read(p)
}

// Every we write to a timeout connection, we advance its write deadline.
func (tr *timeoutConn) Write(p []byte) (n int, err error) {
	var zero time.Duration
	if tr.ReadTimeout != zero {
		tr.Conn.SetReadDeadline(time.Now().Add(tr.ReadTimeout))
	}
	return tr.Conn.Write(p)
}
