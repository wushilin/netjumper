package main

import (
        "github.com/wushilin/netjumper/lib"
        "fmt"
        "io"
        "log"
        "os"
        "sync"
        "sync/atomic"
        "time"
        "net"
)

var SESSION_ID uint64 = 0

func nextSessionId() uint64 {
        return atomic.AddUint64(&SESSION_ID, 1)
}

var SECRET string = ""

func main() {
        if len(os.Args) != 3 {
                fmt.Printf("Usage: %s <port> <secret>\n", os.Args[0])
                os.Exit(1)
        }
	PORT_STR := os.Args[1]
        SECRET = os.Args[2]

        tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%s", PORT_STR))
        if err != nil {
                log.Fatal(err)
        }
        sock, err := net.ListenTCP("tcp", tcpAddr)
        if err != nil {
                log.Fatal(err)
        }

        for {
                sessionId := nextSessionId()
                clientConn, err := sock.AcceptTCP()
                if err != nil {
                        continue
                }
                fmt.Printf("[session=%d] - incoming conn from %v\n", sessionId, clientConn.RemoteAddr())
                go handle(clientConn, sessionId)
        }
}

func handle(clientConn *net.TCPConn, sessionId uint64) {
        startTime := time.Now()
        defer func() {
                fmt.Printf("[session=%d] - closing conn from %v\n", sessionId, clientConn.RemoteAddr())
                clientConn.Close()
                fmt.Printf("[session=%d] - Total time %v\n", sessionId, time.Since(startTime))
        }()
        // first send challenge bytes
        challenge := lib.RandomData(32)
        fmt.Printf("[session=%d] - sending challenge %s\n", sessionId, string(challenge))
        err := lib.WriteData(clientConn, challenge)
        if err != nil {
                return
        }
        response, err := lib.ReadData(clientConn)
        if err != nil {
                return
        }
        fmt.Printf("[session=%d] - Received response % x\n", sessionId, response)
        tocalc := lib.ArrayConcat(challenge, []byte(SECRET))

        expected := lib.Sha1(tocalc)
        if !lib.ArrayEqual(expected, response) {
                lib.WriteByte(clientConn, 1)
                lib.WriteData(clientConn, []byte("challenge failed, check secret"))
                fmt.Printf("[session=%d] - Challenge is wrong\n", sessionId)
                return
        } else {
                lib.WriteByte(clientConn, 0)
                fmt.Printf("[session=%d] - Challenge OK\n", sessionId)
        }

        hostStringBuffer, err := lib.ReadData(clientConn)
        if err != nil {
                fmt.Printf("[session=%d] - failed to read host info\n", sessionId)
                return
        }

        hostString := string(hostStringBuffer)
        remoteConnTmp, err := net.DialTimeout("tcp", hostString, 5*time.Second)
        if err != nil {
                fmt.Printf("[session=%d] - Connect to %s failed: %s\n", sessionId, hostString, err.Error())
                // write byte 1 for failure
                lib.WriteByte(clientConn, 1)
                errorMessage := err.Error()
                lib.WriteData(clientConn, []byte(errorMessage))
                return
        }
	remoteConn := remoteConnTmp.(*net.TCPConn)

        fmt.Printf("[session=%d] - Connected to %s\n", sessionId, hostString)
        lib.WriteByte(clientConn, 0)
        fmt.Printf("[session=%d] - Time taken to establish connection %v\n", sessionId, time.Since(startTime))
        pipe(sessionId, clientConn, remoteConn)
        defer func() {
                fmt.Printf("[session=%d] - closing conn to %s\n", sessionId, hostString)
                remoteConn.Close()
        }()
}

func pipe(sessionId uint64, clientConn *net.TCPConn, remoteConn *net.TCPConn) {
        fmt.Printf("[session=%d] - piping...\n", sessionId)
        wg := sync.WaitGroup{}
        wg.Add(2)
        var sent int64 = 0
        var received int64 = 0

        go pipeOneway(&wg, sessionId, clientConn, remoteConn, "client to server", &received)
        go pipeOneway(&wg, sessionId, remoteConn, clientConn, "server to client", &sent)
        wg.Wait()
        fmt.Printf("[session=%d] - done. sent %d bytes, received %d bytes\n", sessionId, sent, received)
}

func pipeOneway(wg *sync.WaitGroup, sessionId uint64, reader *net.TCPConn, writer *net.TCPConn, direction string, counter *int64) {
        defer func() {
                defer wg.Done()
                defer reader.CloseRead()
                defer writer.CloseWrite()
        }()
        buf := make([]byte, 1024)
        for {
                nread, err := reader.Read(buf)
                if nread > 0 {
                        atomic.AddInt64(counter, int64(nread))
                        _, err = writer.Write(buf[:nread])
                        if err != nil {
                                if err != io.EOF {
                                        fmt.Printf("[session=%d] - Write for %s failed with %s\n", sessionId, direction, err.Error())
                                }
                                return
                        }
                }
                if err != nil {
                        if err != io.EOF {
                                fmt.Printf("[session=%d] - Read from %s failed with %s\n", sessionId, direction, err.Error())
                        }
                        return
                }
        }
}

