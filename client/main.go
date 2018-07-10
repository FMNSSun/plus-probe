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
var MaxPacketSize int = 1024
var ReadTimeout int
var Sleep int
var Burst int
var mutex = &sync.Mutex{}
var Cat uint64

func writeString(w io.Writer, msg string) {
	mutex.Lock()

	io.WriteString(w, msg)

	mutex.Unlock()
}

func main() {
	laddr := flag.String("laddr","localhost:6138","Local addresses to listen on.")
	raddr := flag.String("raddr","localhost:6137","Remote address to connect to.")
	readTimeout := flag.Int("read-timeout", 10, "Read timeout.")
	sleep := flag.Int("sleep", 2, "How long to slep after sending packets.")
	burst := flag.Int("burst", 3, "How many packets to burst send.")
	cat := flag.Int("cat", 0, "CAT to use!")

	flag.Parse()

	ReadTimeout = *readTimeout
	Sleep = *sleep
	Burst = *burst
	Cat = uint64(*cat)

	wOut = os.Stdout

	writeString(wOut, fmt.Sprintf("START\t%d\t%s\t%s\n", time.Now().UnixNano(), *laddr, *raddr))

	for {
		writeString(wOut, fmt.Sprintf("RESET\t%d\n", time.Now().UnixNano()))

		packetConn, err := net.ListenPacket("udp", *laddr)

		if err != nil {
			panic(err.Error())
		}

		udpAddr, err := net.ResolveUDPAddr("udp", *raddr)

		if err != nil {
			panic(err.Error())
		}

		if Cat == 0 {
			Cat = PLUS.RandomCAT()
		}

		connectionManager, conn := PLUS.NewConnectionManagerClient(packetConn, Cat, udpAddr)
		go connectionManager.Listen()

		handleConnection(conn)
	}
}

func handleConnection(conn *PLUS.Connection) {
	recvBuf := make([]byte, MaxPacketSize)
	sendBuf := make([]byte, 640)
	closeChan := make(chan bool)
	addrChan := make(chan net.Addr)

	packetNo := uint64(0)

	cat := conn.CAT()

	recvAddr := ""

	go func() {
		curAddr := conn.RemoteAddr()

		for {
			now := time.Now().UnixNano()

			tout := time.Now().Add(time.Duration(ReadTimeout) * time.Second)
			conn.SetReadDeadline(tout)

			n, addr, err := conn.ReadAndAddr(recvBuf)

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
				addrChan <- addr
			}

			recvString := string(recvBuf[:n])
			writeString(wOut, fmt.Sprintf("DATA\t%d\t%d\t%s\t%q\n", cat, now, curAddr.String(), recvString))
			writeString(wOut, fmt.Sprintf("RECV\t%d\t%d\t%s\t%d\n", cat, now, curAddr.String(), n))

			if recvAddr == "" {
				recvAddr = recvString
			}

			if recvAddr != recvString {
				writeString(wOut, fmt.Sprintf("CHADDR_L\t%d\t%d\t%s\t%q\n", cat, now, curAddr.String(), recvString))
				recvAddr = recvString
			}
		}
	}()

	curAddr := conn.RemoteAddr()
	m := copy(sendBuf, []byte(curAddr.String()))

	for {
		now := time.Now().UnixNano()

		select {
		case _ = <- closeChan:
			conn.Close()
			return
		case addr := <- addrChan:
			curAddr = addr
			m = copy(sendBuf, []byte(curAddr.String()))
		default:

			for i := 0; i < Burst; i++ {
				n, err := conn.Write(sendBuf[:m])
				if err == nil {
					writeString(wOut, fmt.Sprintf("SENT\t%d\t%d\t%s\t%d\n", cat, now, curAddr.String(), n))
				} else {
					writeString(wOut, fmt.Sprintf("ERROR\t%d\t%d\t%s\t%s\n", cat, now, curAddr.String(), err.Error()))
				}
			}

			packetNo++

			time.Sleep(time.Duration(Sleep) * time.Second)
		}
	}
}
