package main

import (
	"bytes"
	"github.com/tak1827/heartbeat"
	"log"
	"net"
	"time"
)

func main() {
	ca, _ := net.ListenPacket("udp", "127.0.0.1:0")
	cb, _ := net.ListenPacket("udp", "127.0.0.1:0")
	ea, err := heartbeat.NewEndpoint(ca, processPacket, check, nil)
	check(err)
	eb, err := heartbeat.NewEndpoint(cb, processPacket, check, nil)
	check(err)

	defer func() {
		check(ca.SetDeadline(time.Now().Add(1 * time.Millisecond)))
		check(cb.SetDeadline(time.Now().Add(1 * time.Millisecond)))

		check(ea.Close())
		check(eb.Close())

		check(ca.Close())
		check(cb.Close())
	}()

	go ea.Listen()
	go eb.Listen()

	for i := 0; i < 32; i++ {
		val := bytes.Repeat([]byte("x"), i*100)
		// val := bytes.Repeat([]byte("x"), 2400)
		check(ea.WritePacket(val, cb.LocalAddr()))
	}
}

func check(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func processPacket(packetData []byte) {
	log.Printf("recv (packetData size=%d)", len(packetData))
}

// func transmitPacket(conn net.PacketConn, addr net.Addr, buf []byte) {
// 	if rand.Intn(100) == 0 {
// 		// 1% packet loss
// 		return
// 	}

// 	n, err := conn.WriteTo(buf, addr)

// 	if err == nil && n != len(buf) {
// 		err = io.ErrShortWrite
// 	}

// 	n, err := conn.Write(buf)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	if n < len(buf) {
// 		log.Fatal("failed to transmit all packet")
// 	}
// }
