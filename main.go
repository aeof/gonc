package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/aeof/gonc/client"
)

var (
	verbose           bool
	timeoutConnection int
	timeoutIdle       int
)

const DefaultTimeout = 0

func init() {
	flag.IntVar(&timeoutIdle, "w", DefaultTimeout, "If a connection and stdin are idle for more than timeout seconds then the connection is silently closed.")
	flag.IntVar(&timeoutConnection, "G", DefaultTimeout, "TCP connection timeout in seconds")
	flag.BoolVar(&verbose, "v", false, "Produce more verbose output.")
	flag.Parse()
}

func checkError(err error) {
	if err == nil {
		return
	}

	// only output error when verbose mode is on
	if verbose {
		fmt.Fprintln(os.Stderr, err)
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
	conn, err := net.DialTimeout("tcp", host+":"+port, time.Duration(timeoutConnection)*time.Second)
	checkError(err)
	defer conn.Close()
	if verbose {
		fmt.Printf("Succeeded to connect to %s %s port!\n", host, port)
	}

	conn = client.NewTimeoutConn(conn, time.Duration(timeoutIdle)*time.Second, time.Duration(timeoutIdle)*time.Second)
	go func() {
		io.Copy(conn, os.Stdin)
	}()
	_, err = io.Copy(os.Stdout, conn)
	checkError(err)
}
