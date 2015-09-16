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
  "bytes"
  "fmt"
)

const keySize = 1024

func TestClientServerProtocol(t *testing.T) {
	connA, connB := NewTestConns()


	var seed1 [keySize]byte
	var seed2 [keySize]byte
	seed2[0] = 1

	keyA, err := rsa.GenerateKey(bytes.NewReader(seed1[:]), keySize)
	if (err != nil) {
		t.Errorf("failed write %s", err)
		return
	}

	keyB, err := rsa.GenerateKey(bytes.NewReader(seed2[:]), keySize)
	if (err != nil) {
		t.Errorf("failed write %s", err)
		return
	}

	var clientConn net.Conn = nil
	var clientErr error = nil

	clientDone := make(chan bool)
	serverDone  := make(chan bool)

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
		clientConn, clientErr = node.HandleServerConn(connA, keyA, &keyB.PublicKey)
		clientDone <- true
	}()

	go func() {
		serverConn, serverErr = node.HandleClientConn(connB, keyB, &contactsList)
		serverDone <- true
	}()

	done := <-clientDone
	done = <-serverDone

	fmt.Println(done)

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
