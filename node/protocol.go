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
  "io/ioutil"
  "bytes"
  "encoding/pem"
  "fmt"

  "github.com/danoctavian/bluntly/netutils"
)


func HandleClientConn(rawConn net.Conn,
                      ownKey *rsa.PrivateKey,
                      peerPubKey *rsa.PublicKey) (conn *Conn, err error) {  
  pubKey, privKey, err := box.GenerateKey(rand.Reader)
  if (err != nil) { return }

  keyHash, err := pubKeyHash(&ownKey.PublicKey)
  if (err != nil) { return }

  connRequest := ConnRequest{keyHash, pubKey}

  reqBytes, err := connRequest.MarshalBinary()
  if (err != nil) { return }

  err = writePubKeyMsg(rawConn, peerPubKey, reqBytes)
  if (err != nil) { return }

  fmt.Println("reading the public key from the pipe")
  plainReply, err := readPubKeyMsg(rawConn, ownKey)
  if (err != nil) { return }

  var peerSessionKey [sessionKeyLen]byte
  copy(peerSessionKey[:], plainReply) 

  var sharedKey [sessionKeyLen]byte
  box.Precompute(&sharedKey, &peerSessionKey, privKey) 

  return &Conn{Conn: rawConn,
              sharedKey: &sharedKey,
              readBuf: netutils.NewCircularBuf(readBufferCapacity),
              readerBufMutex: &sync.Mutex{}}, nil
}

func HandleServerConn(rawConn net.Conn,
                      ownKey *rsa.PrivateKey,
                      contacts *ContactList) (conn *Conn, err error) {

  plain, err := readPubKeyMsg(rawConn, ownKey)

  connReq := ConnRequest{}
  err = connReq.UnmarshalBinary(plain)
  if (err != nil) { return }

  contact := contacts.GetContact(connReq.peerPubHash)
  if (contact == nil) {return nil, ContactNotFoundError{connReq.peerPubHash[:]}}

  pubKey, privKey, err := box.GenerateKey(rand.Reader)
  if (err != nil) { return }

  var sharedKey [sessionKeyLen]byte
  box.Precompute(&sharedKey, connReq.sessionKey, privKey)

  response := pubKey[:]
  err = writePubKeyMsg(rawConn, contact.PublicKey, response)

  return &Conn{Conn: rawConn,
              sharedKey: &sharedKey,
              // TODO: circular buffer capacity may cause 
              // streaming to fail. to avoid,
              // put a cap on the size of the encrypted chunks
              readBuf: netutils.NewCircularBuf(readBufferCapacity),
              readerBufMutex: &sync.Mutex{}}, nil
}

func readPubKeyMsg(rawConn net.Conn, ownKey *rsa.PrivateKey) (plain []byte, err error) {

  var msgLen int64
  err = binary.Read(rawConn, binary.BigEndian, &msgLen)
  if (err != nil) { return }

  ciphertext := make([]byte, msgLen)
  _, err = io.ReadFull(rawConn, ciphertext)
  if (err != nil) { return }

  plain, err = rsa.DecryptPKCS1v15(rand.Reader, ownKey, ciphertext)
  if (err != nil) { return }
  return
}

func writePubKeyMsg(rawConn net.Conn,
                    peerKey *rsa.PublicKey,
                    msg []byte) (err error) {
  encryptedMsg, err := rsa.EncryptPKCS1v15(rand.Reader, peerKey, msg)
  if (err != nil) { return }

  prefixedMsg := lenPrefix(encryptedMsg)

  _, err = rawConn.Write(prefixedMsg)
  return
}

/* connection request */
type ConnRequest struct {
  peerPubHash [32]byte
  sessionKey *[sessionKeyLen]byte 
}

type ConnResponse struct {
  sessionKey *[sessionKeyLen]byte
}

func (r *ConnRequest) MarshalBinary() (data []byte, err error) {
  return append((*r.sessionKey)[:], r.peerPubHash[:]...), nil
}

func (r *ConnRequest) UnmarshalBinary(data []byte) (err error) {
  var sess [sessionKeyLen]byte
  r.sessionKey = &sess
  copiedBytes := copy(r.sessionKey[:], data[:sessionKeyLen])
  if (copiedBytes < sessionKeyLen) {
    return errors.New("session key too short.")    
  }

  copiedBytes = copy(r.peerPubHash[:], data[sessionKeyLen:])
  if (copiedBytes != len(r.peerPubHash[:])) {
    return errors.New("pub key hash too short")
  }
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


  fmt.Println("attempting to read in a buffer of size", len(b))
  readBytes, _ = c.readBuf.Read(b)

  if readBytes > 0 {
    return readBytes, nil
  } // just serve the data from the buffer

  // if the buffer is empty
  msg, err := c.readFromConn()
  if (err != nil) {
    fmt.Println("got read from conn Error ", err)
  }

  readBytes = copy(b, msg)
  if readBytes < len(msg) { 
    // there's data unread that needs to be buffered
    _, err = c.readBuf.Write(msg[readBytes:])
    if (err != nil) {return 0, err}
  }

  fmt.Println("succesfully read this many bytes", readBytes, b[:readBytes])
  return
}

// reads data from the network connection
func (c *Conn) readFromConn() (msg []byte, err error) {
  var msgLen uint64
  fmt.Println("attempting to read from conn")
  err = binary.Read(c.Conn, binary.BigEndian, &msgLen)
  if (err != nil) { return nil, err}

  fmt.Println("read len prefix of message")

  cipherBuf := make([]byte, msgLen)
  _, err = io.ReadFull(c.Conn, cipherBuf)

  if (err != nil) {return nil, err}

  msg, err = Decrypt(cipherBuf, c.sharedKey)

  fmt.Println("read message ", msg)
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
  fmt.Println("encrypting the message ", msg)
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

  msg = make([]byte, 0) 

  msg, success := box.OpenAfterPrecomputation(msg, ciphertext[nonceLen:], &nonce, sharedKey)

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

type ContactNotFoundError struct {
  hash []byte
}

func (e ContactNotFoundError) Error() string {
  return fmt.Sprintf("failed to find contact with pub key hash: [% x]", e.hash)
}

func RsaKeyFromPEM(filename string) (key *rsa.PrivateKey, err error) {

  data, err := ioutil.ReadFile(filename)
  if err != nil { return }

  block, _ := pem.Decode(data)
  return x509.ParsePKCS1PrivateKey(block.Bytes)
}