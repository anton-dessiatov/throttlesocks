package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/thinkgos/go-socks5"
	"golang.org/x/time/rate"
)

func main() {
	var listenAddress = flag.String("l", "", "Address to listen for incoming SOCKS5 connections (for example 'localhost:3218')")
	var limit = flag.String("b", "", "Bandwidth limit in <number><unit> format. Allowed units are GBps, Gbps, MBps, Mbps, KBps, Kbps, Bps, bps")
	flag.Parse()

	if *listenAddress == "" {
		log.Fatal("Please set listenAddress")
	}
	if *limit == "" {
		log.Fatal("Please set limit")
	}

	bps, err := ParseLimit(*limit)
	if err != nil {
		log.Fatal(err)
	}

	limiter := NewLimiter(rate.Limit(bps))
	srv := socks5.NewServer(socks5.WithDial(func(ctx context.Context, network, addr string) (net.Conn, error) {
		netConn, err := net.Dial(network, addr)
		if err != nil {
			return nil, fmt.Errorf("net.Dial: %w", err)
		}
		return NewLimitedConnection(netConn, limiter), nil
	}))

	log.Fatal(srv.ListenAndServe("tcp", *listenAddress))
}
