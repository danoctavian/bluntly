package node

import (
  "errors"
  "github.com/nictuku/dht"
  "net"
  "crypto/rsa"
)

/* NODE */
type Node struct {

  dht *dht.DHT
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
}

type HolePunchConf struct {
	recvPort int
}

func NewNode(conf *Config) (node *Node, err error) {
  // setup the DHT
  dhtConf := dht.DefaultConfig
  dhtConf.Port = conf.dhtPort
  dht, err := dht.New(dhtConf)
  if err != nil { return }
  go dht.Run()

  node.dht = dht

	return nil, errors.New("failed to create node")
}

/* CLIENT */

/* LISTENER */

type Listener struct {
}

type Conn struct {

}

func (n *Node) Listen(port int) (listener Listener, err error) {
  return
}

func (l *Listener) Accept() (c net.Conn, err error) {
  return
}

func (l *Listener) Close() error {
  return nil
}

func Addr() net.Addr {
  return nil
}