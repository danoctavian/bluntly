package node_test

import (
  "testing"
 // "bytes"
  "github.com/danoctavian/bluntly/node"
  "github.com/danoctavian/bluntly/netutils"
  "sync"
  "net"
  "time"
  "crypto/rsa"
  "fmt"
)

const keySize = 1024

func TestClientServerProtocol(t *testing.T) {
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

	var clientConn net.Conn = nil
	var clientErr error = nil

	clientDone := make(chan interface{})
	serverDone  := make(chan interface{})

	var serverConn net.Conn = nil
	var serverErr error = nil

	contacts := map[rsa.PublicKey]string{
		keyA.PublicKey: "some fucker",
	}

	contactsList := node.ContactList{
  	Contacts: &contacts,
  	Mut: &sync.Mutex{},
  }

	go func() {
		clientConn, clientErr = node.HandleClientConn(connA, keyA, &keyB.PublicKey)
		if (clientErr != nil) {
			t.Errorf("client failed with: %s", clientErr)
			return
		}

		clientDone <- true
	}()

	go func() {
		serverConn, serverErr = node.HandleServerConn(connB, keyB, &contactsList)
		if (clientErr != nil) {
			t.Errorf("server failed with: %s", serverErr)
			return
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


type TestConn struct {
	writeMutex *sync.Mutex
	writeBuf *netutils.CircularBuf

	readMutex *sync.Mutex
	readBuf *netutils.CircularBuf	

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
		closedConn: false}

	connB := &TestConn{
		writeMutex: connAReadMutex,
		writeBuf: connABuf,
		readMutex: connBReadMutex,
		readBuf: connBBuf,
		closedConn: false}

	return connA, connB
}

func (c *TestConn) Read(b []byte) (n int, err error) {
	if c.closedConn { return 0, nil }
	c.readMutex.Lock()
	defer c.readMutex.Unlock()

	return c.readBuf.Read(b)
}
func (c *TestConn) Write(b []byte) (n int, err error) {
	if c.closedConn { return 0, nil }
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	return c.writeBuf.Write(b)
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
