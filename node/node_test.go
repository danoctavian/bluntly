package node_test

import (
  "testing"
  "github.com/danoctavian/bluntly/node"
  "golang.org/x/crypto/nacl/box"
  "crypto/rand"
  "bytes"
)

func TestEncryptDecrypt(t *testing.T) {
  peerPubKey, _, err := box.GenerateKey(rand.Reader)
  if (err != nil) { 
    t.Errorf("failed key gen %s", err)
    return
  }

  _, ownPrivKey, err := box.GenerateKey(rand.Reader)
  if (err != nil) { 
    t.Errorf("failed key gen %s", err)
    return
  }

  var sharedKey [32]byte
  box.Precompute(&sharedKey, peerPubKey, ownPrivKey)

  msg := []byte("wtf am i doing")
  
  cipher, err := node.Encrypt(msg, &sharedKey)

  if (err != nil) { 
    t.Errorf("failed to encrypt %s", err)
    return
  }

  plain, err := node.Decrypt(cipher, &sharedKey)
  if (err != nil) { 
    t.Errorf("failed to decrypt: %s", err)
    return
  }

  if (bytes.Equal(msg, plain)) {
    t.Errorf("expected %s doesn't equal actual %s", string(msg), string(plain))
      
  }
}