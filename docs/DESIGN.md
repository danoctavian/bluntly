## DESIGN

need connection initiator and listener at the same time.


connect to:

- supply pubKey
- lookup in DHT
- attempt direct connection
- if fails then lookup announce for rendenzvous
- perform NAT penetration

listen

- announce to DHT
- listen on advertised TCP port
- listen on UDP port
- keep polling for DHT 

screw the UI for now and just make the commandline thing equivalent to NC.
BUT make sure you notify the server about who it's talking to and also ignore 
old commands.

### Lib requirements

* crypto
* DHT implementation (bindings to libtorrent). ipmo
* UTP implementation
* protocol parsers
* UI (cli)

golang has everything

* git@github.com:h2so5/utp.git (unreliable)
* git@github.com:nictuku/dht.git (reasonably reliable)
* https://godoc.org/golang.org/x/crypto/curve25519
* https://github.com/gizak/termui
* parsers are not great but you can write them
