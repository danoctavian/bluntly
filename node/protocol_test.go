package node_test

import (
  "testing"
  "bytes"
  "io"
  "github.com/danoctavian/bluntly/node"
  "github.com/danoctavian/bluntly/netutils"
  "sync"
  "net"
  "time"
  "fmt"
  "errors"
)

const keySize = 1024

func TestClientServerProtocol(t *testing.T) {

	/*
	connA, connB, err := NewTestTCPConns()

	if err != nil {
		t.Errorf("failed to create client/server TCP conns: ", err)
		return
	}
	*/

	connA, connB := NewTestConns()

	keyFile := "../data/mypriv.rsa"

	keyA, err := node.RsaKeyFromPEM(keyFile)
	if (err != nil) {
		t.Errorf("failed loading key file: %s", err)
		return
	}

	keyB, err := node.RsaKeyFromPEM(keyFile)
	if (err != nil) {
		t.Errorf("failed loading key file: %s", err)
		return
	}

	sentByA := [][]byte{[]byte{1, 0, 0, 0, 1}, []byte{2, 0, 0, 0, 2}}
	sentByB := [][]byte{[]byte{3, 0, 0, 0, 3, 4, 7}}//,  bytes.Repeat([]byte{66}, 256)}



	clientDone := make(chan interface{})
	serverDone  := make(chan interface{})

	contactList := node.NewContactList()

	contactList.AddContact(&node.Contact{&keyA.PublicKey})

	go func() {
		var clientConn net.Conn = nil
		var clientErr error = nil
		clientConn, clientErr = node.HandleClientConn(connA, keyA, &keyB.PublicKey)
		if (clientErr != nil) {
			t.Errorf("client failed with: %s", clientErr)
			return
		}

		for _, chunk := range sentByA {
			clientConn.Write(chunk)	
		}

		allBytesFromB := bytes.Join(sentByB[:], []byte{})
		recvBuf := make([]byte, len(allBytesFromB))
		_, err := io.ReadFull(clientConn, recvBuf)

		if (err != nil) {
			t.Errorf("client A failed to receive expected bytes in full")
			return			
		}

		if (!bytes.Equal(recvBuf, allBytesFromB)) {
			t.Errorf("client A did not receive expected bytes")
			return
		}

		clientDone <- true
	}()

	go func() {
		var serverConn net.Conn = nil
		var serverErr error = nil

		serverConn, serverErr = node.HandleServerConn(connB, keyB, contactList)
		if (serverErr != nil) {
			t.Errorf("server failed with: %s", serverErr)
			return
		}

		allBytesFromA := bytes.Join(sentByA[:], []byte{})

		recvBuf := make([]byte, len(allBytesFromA))
		_, err := io.ReadFull(serverConn, recvBuf)

		if (err != nil) {
			t.Errorf("server B failed to receive expected bytes in full")
			return			
		}

		if (!bytes.Equal(recvBuf, allBytesFromA)) {
			t.Errorf("server B did not receive expected bytes")
			return
		}

		for _, chunk := range sentByB {
			_, err:= serverConn.Write(chunk)

			if (err != nil) {
				t.Errorf("failed to send message")
				return
			}
		}

		serverDone <- true
	}()

	fmt.Println("waiting for done")

	_, err = netutils.ReadWithTimeout(serverDone, 1000)

	if (err != nil) {
		t.Errorf("server failed to terminate: %s", err)
		return
	}

	_, err = netutils.ReadWithTimeout(clientDone, 1000)

	if (err != nil) {
		t.Errorf("server failed to terminate: %s", err)
		return
	}
}

// creates a server and a client on localhost and connects them
// to each other
func NewTestTCPConns() (client net.Conn, server net.Conn, err error) {

	port := 34004 // this can fail if the socket is taken
	connChan := make(chan net.Conn)

	go func() {

		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			connChan <- nil
		}
		for {
			conn, err := ln.Accept()
			if err != nil {
				connChan <- nil
			}
			connChan <- conn
		}
	}()

 	// this is not reliable but should do the trick most of the time
	time.Sleep(time.Second)
	client, err = net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return
	}

	server = <-connChan
	if (server == nil) {
		err = errors.New("server connection failed")
		return
	}

	return
}

type TestConn struct {
	writeMutex *sync.Mutex
	writeBuf *netutils.CircularBuf

	readMutex *sync.Mutex
	readBuf *netutils.CircularBuf	

	connId string
	closedConn bool
}

// creates a connection pair (A, B)
// they represent the 2 heads of a network connection
// so they are interwinded. 
func NewTestConns() (*TestConn, *TestConn) {
	connABuf := netutils.NewCircularBuf(2048)
	connBBuf := netutils.NewCircularBuf(2048)

	connAReadMutex := &sync.Mutex{}

	connBReadMutex := &sync.Mutex{}

	connA := &TestConn{
		writeMutex: connBReadMutex,
		writeBuf: connBBuf,
		readMutex: connAReadMutex,
		readBuf: connABuf,
		connId: "ConnA",
		closedConn: false}

	connB := &TestConn{
		writeMutex: connAReadMutex,
		writeBuf: connABuf,
		readMutex: connBReadMutex,
		readBuf: connBBuf,
		connId: "ConnB",
		closedConn: false}

	return connA, connB
}

func (c *TestConn) Read(b []byte) (n int, err error) {
	if c.closedConn { return 0, nil }
	c.readMutex.Lock()
	defer c.readMutex.Unlock()


	n, err = c.readBuf.Read(b)

	isZero := true
	for _, x := range b[:8] {
		if (x != 0) { isZero = false }
	}

	if (len(b) == 8 && !isZero) {
		fmt.Println("reading the big one and the buf is", b)
	}
	return
}
func (c *TestConn) Write(b []byte) (n int, err error) {
	if c.closedConn { return 0, nil }
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

//	fmt.Println("about to put in buf", b)
	n, err = c.writeBuf.Write(b)
//	fmt.Println("state of buf after write", c.writeBuf.GimmeBuf(), err)
	return
}

func (c *TestConn) Close() error {
	c.readMutex.Lock()
	defer c.readMutex.Unlock()
	c.closedConn = true
	return nil
}
func (c *TestConn) LocalAddr() net.Addr {
	return nil
}
func (c *TestConn) RemoteAddr() net.Addr {
	return nil
}
func (c *TestConn) SetDeadline(t time.Time) error {
	return nil
}
func (c *TestConn) SetReadDeadline(t time.Time) error {
	return nil
}
func (c *TestConn) SetWriteDeadline(t time.Time) error {
	return nil
}
