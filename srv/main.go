package main

import (
	"flag"
	"net"
	"time"
	"io"
	"fmt"
	"os"

	"github.com/mami-project/plus-lib"
)

var wOut io.Writer
var ReadTimeout int = 5

func main() {
	laddr := flag.String("laddr","localhost:6137","Local address to listen on.")
	readTimeout := flag.Int("read-timeout",10,"Read timeout.")

	flag.Parse()

	ReadTimeout = *readTimeout

	wOut = os.Stdout

	io.WriteString(wOut, fmt.Sprintf("START\t%d\t%s\n", time.Now().UnixNano(), *laddr))

	packetConn, err := net.ListenPacket("udp", *laddr)

	if err != nil {
		panic(err.Error())
	}

	connectionManager := PLUS.NewConnectionManager(packetConn)
	go connectionManager.Listen()

	for {
		connection := connectionManager.Accept()
		go handleConnection(connection)
	}
}

func handleConnection(conn *PLUS.Connection) {
	buf := make([]byte, PLUS.MaxPacketSize)
	curAddr := conn.RemoteAddr()

	cat := conn.CAT()

	for {
		tout := time.Now().Add(time.Duration(ReadTimeout) * time.Second)
		conn.SetReadDeadline(tout)

		n, addr, err := conn.ReadAndAddr(buf)

		now := time.Now().UnixNano()

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

		n, err = conn.Write(buf[:n])

		io.WriteString(wOut, fmt.Sprintf("SENT\t%d\t%d\t%s\t%d\n", cat, now, curAddr.String(), n))
	}
}