package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

var (
	verbose       bool
	timeoutSecond int
)

const DefaultTimeout = 0

func init() {
	flag.IntVar(&timeoutSecond, "w", DefaultTimeout, "Connections which cannot be established or are idle timeout after timeout seconds.")
	flag.BoolVar(&verbose, "v", false, "Produce more verbose output.")
	flag.Parse()
}

// timeoutConn is a wrapper for net.Conn, which sets deadline for every read/write.
type timeoutConn struct {
	net.Conn
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func newTimeoutConn(conn net.Conn, readTimeout, writeTimeout time.Duration) net.Conn {
	return timeoutConn{
		Conn:         conn,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
	}
}

func (tr timeoutConn) Read(p []byte) (n int, err error) {
	// when read timeout is zero, read will not timeout
	if tr.readTimeout != 0 {
		tr.SetReadDeadline(time.Now().Add(tr.readTimeout))
	}
	return tr.Conn.Read(p)
}

func (tr timeoutConn) Write(p []byte) (n int, err error) {
	// when write timeout is zero, write will not timeout
	if tr.writeTimeout != 0 {
		tr.SetWriteDeadline(time.Now().Add(tr.writeTimeout))
	}
	return tr.Conn.Write(p)
}

func checkError(err error) {
	if err == nil {
		return
	}

	// only output error when verbose mode is on
	if verbose {
		fmt.Fprint(os.Stderr, err)
	}
	os.Exit(1)
}

func main() {
	// parse arguments
	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(1)
	}
	host := args[0]
	port := args[1]

	// connect to the server
	timeout := time.Duration(timeoutSecond) * time.Second
	conn, err := net.DialTimeout("tcp", host+":"+port, timeout)
	checkError(err)
	defer conn.Close()
	if verbose {
		fmt.Printf("Succeeded to connect to %s %s port!\n", host, port)
	}

	conn = newTimeoutConn(conn, timeout, timeout)
	go func() {
		io.Copy(conn, os.Stdin)
	}()
	_, err = io.Copy(os.Stdout, conn)
	checkError(err)
}
