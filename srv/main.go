package main

import (
	"flag"
	"net"
	"time"
	"io"
	"fmt"
	"os"
	"sync"

	"github.com/mami-project/plus-lib"
)

var wOut io.Writer
var ReadTimeout int = 5
var Sleep int = 330
var mutex = &sync.Mutex{}

func writeString(w io.Writer, msg string) {
	mutex.Lock()

	io.WriteString(w, msg)

	mutex.Unlock()
}

func main() {
	laddr := flag.String("laddr","localhost:6137","Local address to listen on.")
	readTimeout := flag.Int("read-timeout",10,"Read timeout.")
	sleep := flag.Int("sleep", 330, "Time to sleep between sending packets (in ms).")

	flag.Parse()

	ReadTimeout = *readTimeout
	Sleep = *sleep

	wOut = os.Stdout

	writeString(wOut, fmt.Sprintf("START\t%d\t%s\n", time.Now().UnixNano(), *laddr))

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
	recvBuf := make([]byte, PLUS.MaxPacketSize)
	sendBuf := make([]byte, 640)

	cat := conn.CAT()
	closeChan := make(chan bool)
	addrChan := make(chan net.Addr)

	go func() {
		curAddr := conn.RemoteAddr()
		for {
			tout := time.Now().Add(time.Duration(ReadTimeout) * time.Second)
			conn.SetReadDeadline(tout)

			n, addr, err := conn.ReadAndAddr(recvBuf)

			now := time.Now().UnixNano()

			if err != nil {
				if err == PLUS.ErrReadTimeout {
					writeString(wOut, fmt.Sprintf("TIMEOUT\t%d\t%d\t%s\n", cat, now, curAddr.String()))
				} else {
					writeString(wOut, fmt.Sprintf("ERROR\t%d\t%d\t%s\t%q\n", cat, now, curAddr.String(), err.Error()))
				}
				closeChan <- true
				return
			}

			if curAddr.String() != addr.String() {
				writeString(wOut, fmt.Sprintf("CHADDR\t%d\t%d\t%s\t%s\n", cat, now, curAddr.String(), addr.String()))
				curAddr = addr
				addrChan <- curAddr
			}

			recvString := string(recvBuf[:n])
			writeString(wOut, fmt.Sprintf("DATA\t%d\t%d\t%s\t%q\n", cat, now, curAddr.String(), recvString))
			writeString(wOut, fmt.Sprintf("RECV\t%d\t%d\t%s\t%d\n", cat, now, curAddr.String(), n))
		}
	}()

	curAddr := conn.RemoteAddr()
	m := copy(sendBuf, []byte(curAddr.String()))

	for {
		now := time.Now().UnixNano()

		select {
		case addr := <- addrChan:
			curAddr = addr
			m = copy(sendBuf, []byte(curAddr.String()))
		case _ = <- closeChan:
			conn.Close()
			return
		default:

			n, err := conn.Write(sendBuf[:m])

			if err == nil {
				writeString(wOut, fmt.Sprintf("SENT\t%d\t%d\t%s\t%d\n", cat, now, curAddr.String(), n))
			} else {
				writeString(wOut, fmt.Sprintf("ERROR\t%d\t%d\t%s\t%s\n", cat, now, curAddr.String(), err.Error()))
			}

			time.Sleep(time.Duration(Sleep) * time.Millisecond)
		}
	}
}
