package client

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeoutConnectionRead(t *testing.T) {
	const serverAddress = "localhost:8080"

	tests := []struct {
		TimeoutClientRead  time.Duration
		LatencyServerWrite time.Duration
		ShouldSuccess      bool
	}{
		{
			TimeoutClientRead:  time.Second,
			LatencyServerWrite: 2 * time.Second,
			ShouldSuccess:      false,
		},
		{
			TimeoutClientRead:  0,
			LatencyServerWrite: 2 * time.Second,
			ShouldSuccess:      true,
		},
		{
			TimeoutClientRead:  time.Second,
			LatencyServerWrite: 500 * time.Millisecond,
			ShouldSuccess:      true,
		},
	}

	// server side sleep for a duration of time before it writes to the client
	listener, err := net.Listen("tcp", serverAddress)
	require.Nil(t, err)
	go func() {
		defer listener.Close()
		for _, test := range tests {
			conn, err := listener.Accept()
			if err != nil {
				t.Log("failed to accept: ", err)
				continue
			}
			defer conn.Close()

			time.Sleep(test.LatencyServerWrite)
			conn.Write([]byte("abcdefg"))
		}
	}()

	buf := make([]byte, 128)
	for _, test := range tests {
		conn, err := net.Dial("tcp", serverAddress)
		require.Nil(t, err)
		defer conn.Close()

		timeoutConn := NewTimeoutConn(conn, test.TimeoutClientRead, 0)
		_, err = timeoutConn.Read(buf)
		require.Equal(t, test.ShouldSuccess, err == nil, err)
	}
}

func TestTimeoutConnectionRepeatedRead(t *testing.T) {
	const serverAddress = "localhost:8080"

	listener, err := net.Listen("tcp", serverAddress)
	require.Nil(t, err)
	go func() {
		defer listener.Close()
		conn, err := listener.Accept()
		if err != nil {
			t.Log("failed to accept: ", err)
			return
		}

		for {
			time.Sleep(time.Second)
			conn.Write([]byte("abcdefg"))
		}
	}()

	conn, err := net.Dial("tcp", serverAddress)
	require.Nil(t, err)
	defer conn.Close()

	buf := make([]byte, 128)
	timeoutConn := NewTimeoutConn(conn, 2500*time.Millisecond, 0)
	for i := 0; i < 3; i++ {
		_, err = timeoutConn.Read(buf)
		require.Nil(t, err)
	}
}
