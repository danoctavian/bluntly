package node

import (
  "github.com/nictuku/dht"
  "net"
  "crypto/rsa"
  "crypto/x509"
  "sync"
  "fmt"
  "io"
  "encoding/binary"
  "errors"
  "golang.org/x/crypto/nacl/box"
  "crypto/rand"
  "crypto/sha1"
  "bytes"
  "encoding/hex"
  "time"

  "github.com/danoctavian/bluntly/stream"
)

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
  contacts *map[rsa.PublicKey]string
  mut *sync.Mutex
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

func handleClientConn(rawConn net.Conn,
                      ownKey *rsa.PrivateKey,
                      contacts *ContactList) (conn *Conn, err error) {

  var handshakeLen int64
  err = binary.Read(rawConn, binary.BigEndian, &handshakeLen)
  if (err != nil) { return }

  ciphertext := make([]byte, handshakeLen)
  _, err = io.ReadFull(rawConn, ciphertext)
  if (err != nil) { return }

  plain, err := rsa.DecryptPKCS1v15(nil, ownKey, ciphertext)
  if (err != nil) { return }

  connReq := ConnRequest{}
  err = connReq.UnmarshalBinary(plain)
  if (err != nil) { return }

  _, privKey, err := box.GenerateKey(rand.Reader)
  if (err != nil) { return }

  var sharedKey [sessionKeyLen]byte
  box.Precompute(&sharedKey, connReq.sessionKey, privKey) 

  return &Conn{Conn: rawConn,
              sharedKey: &sharedKey,
              // TODO: circular buffer capacity may cause 
              // streaming to fail. to avoid,
              // put a cap on the size of the encrypted chunks
              readBuf: stream.NewCircularBuf(2048),
              readerBufMutex: &sync.Mutex{}}, nil
}

const sessionKeyLen = 32

/* connection request */
type ConnRequest struct {
  peerPub *rsa.PublicKey
  sessionKey *[sessionKeyLen]byte 
}

type ConnResponse struct {

}

func (r *ConnRequest) MarshalBinary() (data []byte, err error) {
  pubKeyBytes, err := x509.MarshalPKIXPublicKey(r.peerPub)
  if (err != nil) { return }

  return append((*r.sessionKey)[:], pubKeyBytes...), nil
}

func (r *ConnRequest) UnmarshalBinary(data []byte) (err error) {
  copiedBytes := copy(r.sessionKey[:], data[:sessionKeyLen])
  if (copiedBytes < sessionKeyLen) {
    return errors.New("session key too short.")    
  }

  someKey, err := x509.ParsePKIXPublicKey(data)
  pubKey := someKey.(*rsa.PublicKey)
  if (err != nil) { return }
  r.peerPub = pubKey

  return
}

/* CONNECTION */
type Conn struct {
  net.Conn // underlying network connection 
  sharedKey *[sessionKeyLen]byte

  // a buffer that is used to store in excess data, not yet read
  // but already decrypted
  readBuf *stream.CircularBuf
  readerBufMutex *sync.Mutex
}

func (c *Conn) Read(b []byte) (readBytes int, err error) {
  c.readerBufMutex.Lock()
  defer c.readerBufMutex.Unlock()

  readBytes, _ = c.readBuf.Read(b)
  if readBytes > 0 { return readBytes, nil } // just serve the data from the buffer

  // if the buffer is empty
  msg, err := c.readFromConn()
  readBytes = copy(b, msg)
  if readBytes < len(msg) { 
    // there's data unread that needs to be buffered
    _, err = c.readBuf.Write(msg[readBytes:])
    if (err != nil) {return 0, err}
  }

  return
}

// reads data from the network connection
func (c *Conn) readFromConn() (msg []byte, err error) {
  var msgLen uint64
  err = binary.Read(c.Conn, binary.BigEndian, &msgLen)
  if (err != nil) { return nil, err}

  cipherBuf := make([]byte, msgLen)
  _, err = io.ReadFull(c.Conn, cipherBuf)
  if (err != nil) {return nil, err}
  msg, err = Decrypt(cipherBuf, c.sharedKey)
  return
}

func (c *Conn) Write(msg []byte) (n int, err error) {
  ciphertext, err := Encrypt(msg, c.sharedKey)
  if err != nil {return 0, err}
  final := lenPrefix(ciphertext)

  return c.Conn.Write(final) 
}

func (c Conn) Close() error {
  return nil
}

const nonceLen = 24

func CiphertextLength(msgLen int) int {
  return box.Overhead + nonceLen + msgLen
}

func MsgLength(cipherLen int) int {
  return cipherLen - box.Overhead - nonceLen
}


func Encrypt(msg []byte, sharedKey *[sessionKeyLen]byte) (ciphertext []byte, err error) {
  err = nil
  ciphertext = make([]byte, nonceLen)
  var nonce [nonceLen]byte
   _, err = rand.Read(ciphertext[:nonceLen])
  if err != nil { return }

  copy(nonce[:], ciphertext[:nonceLen])

  ciphertext = box.SealAfterPrecomputation(ciphertext, msg, &nonce, sharedKey)
  return
}

func Decrypt(ciphertext []byte, sharedKey *[sessionKeyLen]byte) (msg []byte, err error) {
  nonceSlice := ciphertext[:nonceLen]
  var nonce [nonceLen]byte
  copy(nonce[:], nonceSlice)

  msgLen := MsgLength(len(ciphertext))
  msg = make([]byte, msgLen) 

  _, success := box.OpenAfterPrecomputation(msg, ciphertext[nonceLen:], &nonce, sharedKey)

  if !success {
    return nil, DecryptError{}
  } else {
    return msg, nil
  }
}

// prefix a buffer with its length
func lenPrefix(b []byte) ([]byte) {
  buf := new(bytes.Buffer)
  binary.Write(buf, binary.BigEndian, uint64(len(b)))
  return append(buf.Bytes(), b...)
}

type DecryptError struct {
}
func (e DecryptError) Error() string { return "failed to decrypt message."}

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
