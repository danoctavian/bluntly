package node

import (
  "net"
  "crypto/rsa"
  "crypto/x509"
  "sync"
  "io"
  "encoding/binary"
  "errors"
  "golang.org/x/crypto/nacl/box"
  "crypto/rand"
  "bytes"

  "github.com/danoctavian/bluntly/netutils"
)


func handleServerConn(rawConn net.Conn,
                      ownKey *rsa.PrivateKey,
                      peerPubKey *rsa.PublicKey) (conn *Conn, err error) {
  return nil, nil
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
              readBuf: netutils.NewCircularBuf(readBufferCapacity),
              readerBufMutex: &sync.Mutex{}}, nil
}

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
  readBuf *netutils.CircularBuf
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
