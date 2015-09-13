package node

import (
  "github.com/nictuku/dht"
  "net"
  "crypto/rsa"
  "sync"
  "fmt"
  "time"
)

/* NODE */
type Node struct { 
  dht *dht.DHT
  config *Config
}

/* CONFIG */
type FileConfig struct {
  dhtPort int
  ownKeyFile string
  id string
  configRoot string
  holePunch HolePunchConf
}

type Config struct {
  dhtPort int
  ownKey rsa.PrivateKey
  id string
  configRoot string
  holePunch HolePunchConf
  contactList *ContactList
}

type HolePunchConf struct {
	recvPort int
}

type ContactList struct {
  contacts *map[string]rsa.PublicKey
  mut *sync.Mutex
}

func NewNode(conf *Config) (node *Node, err error) {
  // setup the DHT
  dhtConf := dht.DefaultConfig
  dhtConf.Port = conf.dhtPort
  dht, err := dht.New(dhtConf)
  if err != nil { return }
  go dht.Run()

  node.dht = dht

	return node, nil
}

/* CLIENT */

/* LISTENER */

type Listener struct {
  connChan chan Conn
}

func (n *Node) Listen(port int) (listener *Listener, err error) {
  connChan := make(chan Conn)
  listener = &Listener{connChan: connChan}
  // setup TCP listener
  tcpListener, err := net.Listen("tcp", fmt.Sprintf(":"))
  if err != nil {return}

  // loop accepting TCP connections
  go func() {
    for {
      tcpConn, tcpErr := tcpListener.Accept()
      if tcpErr != nil {
       Log(LOG_ERROR, "%s", tcpErr)
      }

      go func() {
        conn, handshakeError := handleClientConn(tcpConn, &n.config.ownKey, n.config.contactList)
        if err != nil {
          Log(LOG_INFO,
              "handling client connection from address %s %s",
              tcpConn.RemoteAddr().String(), handshakeError)
        } else {
          connChan <- conn
        }
      }()    
    }
  }()

  return
}

func (l *Listener) Accept() (c net.Conn, err error) {
  conn := <- l.connChan
  return conn, nil
}

func (l *Listener) Close() error {
  return nil
}

func Addr() net.Addr {
  return nil
}

func handleClientConn(conn net.Conn, ownKey *rsa.PrivateKey, contacts *ContactList) (Conn, error) {
  

  return Conn{conn}, nil
}

/* CONNECTION */

type Conn struct {
  net.Conn // underlying network connection 
}

func (c Conn) Read(b []byte) (n int, err error) {
  return
}

func (c Conn) Write(b []byte) (n int, err error) {
  return 
}

func (c Conn) Close() error {
  return nil
}


func (c Conn) LocalAddr() net.Addr {
  return nil
}

// RemoteAddr returns the remote network address.
func (c Conn) RemoteAddr() net.Addr {
  return nil
}

// A zero value for t means I/O operations will not time out.
func (c Conn) SetDeadline(t time.Time) error {
  return nil
}

// SetReadDeadline sets the deadline for future Read calls.
// A zero value for t means Read will not time out.
func (c Conn) SetReadDeadline(t time.Time) error {
  return nil
}

// SetWriteDeadline sets the deadline for future Write calls.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (c Conn) SetWriteDeadline(t time.Time) error {
  return nil
}