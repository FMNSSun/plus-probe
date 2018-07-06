package main

import (
	"flag"
	"net"
	"time"
	"io"
	"fmt"
	"os"
	"encoding/binary"
	"math/rand"

	"github.com/mami-project/plus-lib"
)

var wOut io.Writer
var MaxPacketSize int = 1024
var ReadTimeout int = 5
var Sleep int = 1

func main() {
	laddr := flag.String("laddr","localhost:6138","Local address to listen on.")
	raddr := flag.String("raddr","localhost:6137","Remote address to connect to.")
	readTimeout := flag.Int("read-timeout", 10, "Read timeout.")
	sleep := flag.Int("sleep", 2, "How long to slep after sending a packet.")

	flag.Parse()

	ReadTimeout = *readTimeout
	Sleep = *sleep

	wOut = os.Stdout

	io.WriteString(wOut, fmt.Sprintf("START\t%d\t%s\t%s\n", time.Now().UnixNano(), *laddr, *raddr))

	for {
		io.WriteString(wOut, fmt.Sprintf("RESET\t%d\n", time.Now().UnixNano()))

		packetConn, err := net.ListenPacket("udp", *laddr)

		if err != nil {
			panic(err.Error())
		}

		udpAddr, err := net.ResolveUDPAddr("udp", *raddr)

		if err != nil {
			panic(err.Error())
		}

		connectionManager, conn := PLUS.NewConnectionManagerClient(packetConn, PLUS.RandomCAT(), udpAddr)
		go connectionManager.Listen()

		handleConnection(conn)
	}
}

func handleConnection(conn *PLUS.Connection) {
	buf := make([]byte, MaxPacketSize)
	curAddr := conn.RemoteAddr()

	packetNo := uint64(0)

	cat := conn.CAT()

	for {
		now := time.Now().UnixNano()

		binary.LittleEndian.PutUint64(buf, packetNo)

		for i := 0; i < MaxPacketSize; i++ {
			buf[i] = byte(rand.Intn(256))
		}

		n, err := conn.Write(buf)

		io.WriteString(wOut, fmt.Sprintf("SENT\t%d\t%d\t%s\t%d\n", cat, now, curAddr.String(), n))

		tout := time.Now().Add(time.Duration(ReadTimeout) * time.Second)
		conn.SetReadDeadline(tout)

		n, addr, err := conn.ReadAndAddr(buf)

		if err != nil {
			if err == PLUS.ErrReadTimeout {
				io.WriteString(wOut, fmt.Sprintf("TIMEOUT\t%d\t%d\t%s\n", cat, now, curAddr.String()))
			} else {
				io.WriteString(wOut, fmt.Sprintf("ERROR\t%d\t%d\t%s\t%q\n", cat, now, curAddr.String(), err.Error()))
			}
			conn.Close()
			return
		}

		if curAddr.String() != addr.String() {
			io.WriteString(wOut, fmt.Sprintf("CHADDR\t%d\t%d\t%s\t%s\n", cat, now, curAddr.String(), addr.String()))
			curAddr = addr
		}

		io.WriteString(wOut, fmt.Sprintf("RECV\t%d\t%d\t%s\t%d\n", cat, now, curAddr.String(), n))

		packetNo++

		time.Sleep(time.Duration(Sleep) * time.Second)
	}
}
