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
	verbose bool
	timeout int
)

const DefaultTimeout = 0

func init() {
	flag.IntVar(&timeout, "w", DefaultTimeout, "Connections which cannot be established or are idle timeout after timeout seconds.")
	flag.BoolVar(&verbose, "v", false, "Produce more verbose output.")
	flag.Parse()
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
	timeout := time.Duration(timeout) * time.Second
	conn, err := net.DialTimeout("tcp", host+":"+port, timeout)
	checkError(err)
	defer conn.Close()
	if verbose {
		fmt.Printf("Succeeded to connect to %s %s port!\n", host, port)
	}

	go func() {
		io.Copy(conn, os.Stdin)
	}()
	_, err = io.Copy(os.Stdout, conn)
	checkError(err)
}
