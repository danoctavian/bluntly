package node

import (
  "github.com/nictuku/dht"
  "net"
  "crypto/rsa"
  "crypto/x509"
  "sync"
  "fmt"
  "crypto/sha1"
  "encoding/hex"
  "time"

  "github.com/danoctavian/bluntly/netutils"
)

/* CONSTANTS */

const sessionKeyLen = 32
const nonceLen = 24
const readBufferCapacity = 2048

// size of the buffer for peer notifications
const peerChanBufferCapacity = 100

const tcpPeerConnTimeout = time.Duration(5) * time.Second

/* NODE */
type Node struct { 
  dht *DHT
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
  ownKey *rsa.PrivateKey
  id string
  configRoot string
  holePunch HolePunchConf
  contactList *ContactList
}

type HolePunchConf struct {
	recvPort int
}

type ContactList struct {
  Contacts *map[rsa.PublicKey]string
  Mut *sync.Mutex
}

func NewNode(conf *Config) (node *Node, err error) {
  // setup the DHT
  dhtConf := dht.DefaultConfig
  dhtConf.Port = conf.dhtPort
  dhtClient, err := dht.New(dhtConf)
  if err != nil { return }
  dht := newDHT(dhtClient)

  go dht.Run()

  node.dht = dht

	return node, nil
}

/* CLIENT */
func (n *Node) Dial(peerPubKey *rsa.PublicKey) (conn Conn, err error) {
  infoHash, err := pubKeyToInfoHash(peerPubKey)
  if (err != nil) {return }

  peerNotifications := make(chan string, peerChanBufferCapacity)
  n.dht.subscribeToInfoHash(dht.InfoHash(infoHash), peerNotifications)

  // ask for the infohash
  n.dht.PeersRequest(infoHash, false)

  connSuccessChan := make(chan interface{})

  for peerNotification := range peerNotifications {
    go func(peerNotification string) {

      // ASSUMPTION: peerNotifications doesn't yield duplicates.
      someConn, connErr := n.handlePotentialPeer(peerNotification, peerPubKey)

      if (err != nil) {
        Log(LOG_INFO, "Failed to connect to potential peer %s: %s", peerNotification, connErr)
      }

      connSuccessChan <- someConn
    }(peerNotification)
  }

  maybeConn, err := netutils.ReadWithTimeout(connSuccessChan, 30000)
  if (err != nil) { return }

  return maybeConn.(Conn), nil
}

func (n *Node) handlePotentialPeer(peerAddr string,
                                   peerPubKey *rsa.PublicKey) (conn *Conn, err error) {
  netConn, err := n.establishNetConn(peerAddr)
  if (err != nil) {
    Log(LOG_WARN, "Failed to setup net connection to %s: %s", peerAddr, err)
    return
  }
  return HandleServerConn(netConn, n.config.ownKey, peerPubKey)
}

// establish network communication
func (n *Node) establishNetConn(peerAddr string) (conn net.Conn, err error) {
  // attempt a direct TCP connection first

  Log(LOG_DEBUG, "Attempting direct TCP connection to peer %s.")

  tcpConn, tcpErr := net.DialTimeout("tcp", peerAddr, tcpPeerConnTimeout)

  if (err != nil) {
    Log(LOG_WARN, "Failed direct TCP connection to peer %s: %s", peerAddr, tcpErr)    
  }
  Log(LOG_DEBUG, "Attempting UDP connection to peer %s.", peerAddr) 

  return tcpConn, tcpErr
}

/* LISTENER */
type Listener struct {
  connChan chan *Conn
}

func (n *Node) Listen(port int) (listener *Listener, err error) {
  connChan := make(chan *Conn)
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
        conn, handshakeError := HandleClientConn(tcpConn, n.config.ownKey, n.config.contactList)
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

  // announce yourself to the DHT
  go n.announceSelf()

  return
}

func (n *Node) announceSelf() error {
  infoHash, err := pubKeyToInfoHash(&n.config.ownKey.PublicKey)
  if (err != nil) {return err}

  for {
    // the peers request is done only for the purpose of announcing
    n.dht.PeersRequest(infoHash, true) 
    time.Sleep(10 * time.Second)
  }
}

func pubKeyToInfoHash(pub *rsa.PublicKey) (string, error) {
  pubKeyBytes, err := x509.MarshalPKIXPublicKey(pub)
  if err != nil { return "", err}
  sha1Bytes := sha1.Sum(pubKeyBytes)
  return hex.EncodeToString(sha1Bytes[:]), nil
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

type PeerChan (chan<- string)
/* DHT */
// an enhanced version of the DHT that allows per infohash subscriptions
// with notifications sent down channels
type DHT struct {
  *dht.DHT

  subscribers *map[dht.InfoHash](*ChanSet)
  subMutex *sync.Mutex
}


// holy mother of god... no set type and no generics
type ChanSet struct {
    set map[PeerChan]bool
}

func (set *ChanSet) Add(ch PeerChan) bool {
    _, found := set.set[ch]
    set.set[ch] = true
    return !found 
}
func (set *ChanSet) Remove(ch PeerChan) {
  delete(set.set, ch)
}

func newDHT(dhtClient *dht.DHT) *DHT {
  mp := make(map[dht.InfoHash](*ChanSet))
  return &DHT{dhtClient, &mp, &sync.Mutex{}}
}

func (d *DHT) Run() {
  // launch the goroutine that deals with dispatching notifications about peers
  go d.drainResults()

  // run the underlying dht client normally
  d.DHT.Run()
}

// subscribe for peer notifications on that infohash
func (d *DHT) subscribeToInfoHash(infoHash dht.InfoHash, notificationChan PeerChan) {
  d.subMutex.Lock()
  defer d.subMutex.Unlock()
  chanSet := (*d.subscribers)[infoHash]
  if (chanSet == nil) {
    chanSet = &ChanSet{}
    (*d.subscribers)[infoHash] = chanSet
  }
  chanSet.Add(notificationChan)
}

func (dht *DHT) unsubscribeToInfoHash(infoHash dht.InfoHash, notificationChan PeerChan) {
  dht.subMutex.Lock()
  defer dht.subMutex.Unlock()

  chanSet := (*dht.subscribers)[infoHash]
  if (chanSet != nil) {
    chanSet.Remove(notificationChan)
  }
}

func (dht *DHT) drainResults() {
  for batch := range dht.PeersRequestResults {
    dht.notifyOfPeers(batch)
  }
}

func (dht *DHT) notifyOfPeers(batch map[dht.InfoHash][]string) {
  dht.subMutex.Lock()
  defer dht.subMutex.Unlock()

  for infoHash, peers := range batch {
    for _, peer := range peers {
      peerChanSet := (*dht.subscribers)[infoHash]
      for peerChan, _ := range peerChanSet.set {
        // do something with e.Value
        peerChan <- peer
      }
    }
  }
}
