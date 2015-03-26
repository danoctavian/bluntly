var R = require("ramda")
var DHT = require("bittorrent-dht")
var crypto = require('crypto')
var nacl = require("tweetnacl")
var RSA = require("node-rsa")
var fs = require("fs")
var net = require("net")
var path = require("path")
var jls = require("./json-line-stream")
var streamify = require("stream-array")
var mapS = require("map-stream")
var os = require("os")
var merge = require("merge")
var pipe = require("multipipe")
var StringDecoder = require('string_decoder').StringDecoder
var stream = require('stream')
var util = require('util')
var dgram = require("dgram")

var utp = require("./utp")
var punch = require("./hole-punch")

// "CONSTANTS"

var CIPHER_ENC = 'base64'


/*

example use:

# have a blunt-conf.json 

# server listen
blunt-speak -s 1234 --config=blunt-conf.json

# client connect
blunt-speak -c targetID --config=blunt-conf.json

 it's similar to NC in functionality for now
*/


// why this? because fuck me
// if you use the socket directly as a writable, only the first write succeeds
// the rest just don't happen. <3 nodejs
function SocketStream (socket) {
  stream.Writable.call(this)
  this.ownSocket = socket
}
util.inherits(SocketStream, stream.Writable)
SocketStream.prototype._write = function (chunk, encoding, done) {
  console.log("sending down the socket " + chunk)
  this.ownSocket.write(chunk)
  done()
}

var argv = require('minimist')(process.argv.slice(2))

var config = loadConfig(argv)


// RUDE chat protocol
// just a chatting protocol
// it's not a proper chat program. you can get just 1 connection at 1 time as a server
// everyone else is just ignored. it's a POC, ok
if (argv.c) {
  console.log("running a client")
  var protocol = function (err, conn) {
    if (err) {
      console.log("connection failed with " + err)
      return 
    }
    console.log("succeeded connecting to server!! start chatting.")
    conn.src.pipe(process.stdout) 
    conn.sink.write("i am the client. obey.")
    process.stdin.pipe(mapSSync(function(b) {return b.toString('utf8')})).pipe(conn.sink)
  }
} else if (argv.s) {
  console.log("running a server")
  var alreadyChatting = false
  var protocol = function (conn) {
    if (alreadyChatting) return // if someone else tries to connect just ignore them (RUDE)
    alreadyChatting = true
    console.log("got connection from client!! start chatting.")
    conn.src.pipe(process.stdout) 
    conn.sink.write("i am the server. obey.")
    process.stdin.pipe(mapSSync(function(b) {return b.toString('utf8')})).pipe(conn.sink)
  }
}

run(config, protocol)

function run(config, handleConn) {
  if (fs.existsSync(config.dhtCacheFile)) { // read cached routing table
    console.log("loading routing table from cache...")
    var cacheFile = fs.readFileSync(config.dhtCacheFile)
    bootstrap = JSON.parse(cacheFile)
    var dht = new DHT({bootstrap: bootstrap})
  } else {
    var dht =  new DHT()
  }
  
  augmentDHT(dht)

  dht.listen(config.dhtPort, function() {
    console.log("DHT is listening...")
  })

  dht.on("ready", function() {
    console.log("DHT is ready...")
    process.on('SIGINT', function() { // this is a hack - do it on exit
      // when exiting for whatever reason at this point
      // just dump the routing table for future use
      console.log("dumping dht routing table to file...")
      fs.writeFileSync(config.dhtCacheFile, JSON.stringify(dht.toArray()))
      process.exit()
    })

    // LISTEN
    if (config.listen) {
      var onConn = function(socket) { // handling a socket connection
        console.log("got a connection from a client... ")
        var onChatConn = function(err, conn) { 
          // handling a post-handshake connection
          // which may fail
          if (err) {
            console.log("connection failed hanshake with " + err)
            return
          }
          handleConn(conn)
        }
        recvConn(socket, config.ownKey, config.contacts, onChatConn)
      }
      var key = config.ownKey.pub.exportKey('pkcs8-public-der')
      var infoHash = keyFingerprint(key)
      console.log("advertising yourself with infohash " + infoHash)

      // announcing regularly 
      setInterval(function() {
//        console.log("announcing to " + infoHash + " with port " + config.listen.port)
        dht.announce(infoHash, config.listen.port, function(outcome) {
        })
      }, 10000)

      tcpListen({port: config.listen.port}, onConn)
      if (config.holePunch) {
        udpListen(dht, {port: config.listen.port, ownKey: config.ownKey}, onConn)
      }
    } 


    // CONNECT
    if (config.connect) {
      console.log("connecting...")
      var onConn = function(err, socket) {
        if (err) {
          console.log("failed to initiate a network connection. exiting.")
          return
        }
        console.log("attempting a blunt handshake negotiation..." )
        openConn(socket, config.ownKey, config.ID, config.connect.key, handleConn)
      }
      connect(config.connect.key, dht, config.holePunch, onConn) 
    }
  })
}


// assume dht is ready
function tcpListen(config, handleConn) {

  var tcpServer = net.createServer(handleConn)
  tcpServer.listen(config.port) 
}

/* 
we constantly poll to see if there's new peers 
wanting to connect by querying the DHT for a certain infohash

once we detect a peer, we attempt to hole punch to it
*/

function udpListen(dht, config, handleConn) {

  var server = utp.createServer(handleConn)
  var socket = dgram.createSocket('udp4')
  socket.bind(config.port)
  server.listenSocket(socket, function () {console.log("utp server is listening...")})

  var infoHash = rendezvousInfoHash(config.ownKey.pub)

  punchAttempts = {}

  dht.onInfoHashPeer(infoHash, function(addr, from) {
    if (punchAttempts[addr]) return // we're already working on it
    console.log("attempting a hole punch to potential peer " + addr)
    punchAttempts[addr] = true
    addr = addr.split(":")

    punch.udpHolePunch(socket, {host: addr[0], port: addr[1]}, function(err) {
      if(err) {
        console.log("Hole punch failed with error " + err)
        // leave the node in punchAttempts so you don't retry, at least for this session
      }
      // if the hole punch was succesful, there nothing you need to do
      // the client is required to open the connection

      //let it cool down and then allow for other attempts to connect
      // this will cause the server to keep attempting hole punches every time
      // it learns about the peer
      // TODO: cleanup the above somehow
      // hacky as fuck, i know, don't blame the player, blame the game
      setTimeout(function() {delete punchAttempts[addr]}, 10000)
    }, 15000)
  })

  console.log("looking up peers at the rendezvous infohash " + infoHash)

  var lookupForever = function () {
    // not using the result of this lookup; using the peers that come with on('peer')
    setTimeout(function() { 
      dht.lookup(infoHash, lookupForever)
    }, 5000) 
  }

  dht.lookup(infoHash, lookupForever)
}

function udpConnect(host, port, key, dht, recvPort, handleConn) {
  var infoHash = rendezvousInfoHash(key)      
  console.log("announcing itself on the rendezvous infohash " + infoHash + " with port " + recvPort.val)

  var ownPort = recvPort.val
  recvPort.val++ // use a bigger port for the next one

  dht.announce(infoHash, ownPort, function(outcome) {
    var socket = dgram.createSocket('udp4')
    socket.bind(ownPort)

    console.log("hole punching to " + host + ":" + port)

    punch.udpHolePunch(socket, {host: host, port: port}, function(err) {
      if (err) {
        handleConn(err)
        return
      }
      var client = utp.connect(socket, port, host)
      socket.emit("listening")

      setImmediate(function() { handleConn(null, client) })
    }, Math.pow(10, 6)) // try for a long time...
  })
}

/* a hash where a peer announces that it wants to talk to the listening peer */
function rendezvousInfoHash(pubKey) {
  // we are doing an arbitrary transform of the key, in this case reversal
  // to derive an infohash where peers announce themselves in order to be contacted
  // by the server
  var revKey = reverseBuf(pubKey.exportKey('pkcs8-public-der'))
  return keyFingerprint(revKey)
}

/*
finds out the IP of the peer
and attemtps to make connection to it
*/
function connect(key, dht, holePunch, handleConn) {
  var infoHash = keyFingerprint(key.exportKey('pkcs8-public-der'))
  console.log("atempting connect to infohash " + infoHash)

   // the recv port for udp hole punching
  // we instantiate a mutable location for it so it can be incremented
  if (holePunch) var recvPort = {val: holePunch.recvPort}
  var candidates = {}
  dht.onInfoHashPeer(infoHash, function(addr, from) {
    if (candidates[addr]) return
    candidates[addr] = true
    console.log("attempting a connection to potential peer " + addr)
    addr = addr.split(":") 
    var host = addr[0]
    var port = addr[1]

    console.log(host + " " + port)
    var tcpSuccess = false 
    // attempt a direct connection first with TCP
    var client = net.connect({port: port, host: host}, function() {
      tcpSuccess = true
      handleConn(null, client) // handle connection, no errors
    })

    // if it fails go for a UDP hole punching connection
    // using UTP over UDP for reliability
    var onFail =  function(err) {
      console.log("direct TCP connection attempt failed with error " + err)
      if (holePunch) {
        console.log("Trying a hole punching connection over UDP on the same port.")
        udpConnect(host, port, key, dht, recvPort, handleConn)
      } else {
        handleConn(new Error("Failed to connect directly. making no other attempts."))
      }
    }

    client.setTimeout(3000, function() {
      if (tcpSuccess) return
      console.log("timeout occured")
      onFail(new Error("timeout"))
      client.destroy()
    })
    client.on("error",onFail)
  })

  // trigger the lookup
  dht.lookup(infoHash)
}

function makeConnection(host, port, handleConn) {
}

function loadConfig(argv) {

  var confFilePath = "./blunt-conf.json" // default
  if (argv.config) {confFilePath = argv.config}
  
  var confFile = JSON.parse(fs.readFileSync(confFilePath))
  var config = {}
  config.dhtPort = confFile.dhtPort
  config.ID = confFile.ID
  config.dhtCacheFile = "dht.cache" //default
  config.holePunch = confFile.holePunch
  if (confFile.dhtCacheFile) config.dhtCacheFile = confFile.dhtCacheFile
  
  config.ownKey = { pub: keyFromFile(confFile.ownKey.pub)
                  , priv: keyFromFile(confFile.ownKey.priv)}
  
  // load contacts from directory
  config.contacts = parseContacts(confFile.contactsDir)

  
  if (argv.s) {
    config.listen = {port: argv.s}  
  }
  
  if (argv.c) {
    config.connect = {key: config.contacts[(argv.c)]}  
  }

  return config
}


function parseContacts(dir) {
  // the index is a map contact id -> keyFile
  var index = JSON.parse(fs.readFileSync(path.join(dir, "index.json"))) 

  var contacts = {}
  for (contact in index) {
    contacts[contact] = keyFromFile(path.join(dir, index[contact]))
  }
  return contacts
}

// CRYPTO

function keyFromFile(file) { return new RSA(fs.readFileSync(file))}

function keyFingerprint(key) {
  var shasum = crypto.createHash('sha1')
  shasum.update(key)
  return shasum.digest('hex')
}


function openConn(socket, ownKey, ownID, remoteKey, onConn) {
  var myKeys = nacl.box.keyPair()
  socket.once('data', function(msg) {

    msg = handleFstMessage(msg, socket)

    var response = ownKey.priv.decrypt(msg, 'json')
    response.pubKey = new Uint8Array(objToArray(response.pubKey))

    var shared = nacl.box.before(response.pubKey, myKeys.secretKey) 

    setImmediate(onConn, null, connWithSocket(socket, conn(cryptoOps(shared))))
  })

  var cryptedResp = remoteKey.encrypt({id: ownID, pubKey: myKeys.publicKey}, CIPHER_ENC)
  socket.write(lenPrefix(cryptedResp))
}

function recvConn(socket, ownKey, contacts, onConn) {
  socket.once('data', function (msg) {
    // attempt decryption  
    var msg = handleFstMessage(msg, socket)
    var cryptoOps = makeCrypto(ownKey, contacts, msg)

    if (cryptoOps) {
      socket.write(lenPrefix(cryptoOps.response), function () {
        // this is the shittiest hack yet, i'm sorry
        // it's here so on *most* occasions the initial message 
        // arrives in a different chunk from the next 1
        // TODO: it breaks here; please fix!!!!!!!!
        setTimeout(onConn, 100, null, connWithSocket(socket, conn(cryptoOps)))
      })

    } else {
      socket.end()
      onConn(new Error("handshake failed"))    
    }
  })
}

// produce crypto operations given initial handshake
function makeCrypto(ownKey, contacts, fstData) {

//  try {
    var request = ownKey.priv.decrypt(fstData, 'json')
    request.pubKey = new Uint8Array(objToArray(request.pubKey))

    var rsaPubKey = contacts[request.id]

    // generating a shared secret 
    var myKeys = nacl.box.keyPair()

    var shared = nacl.box.before(request.pubKey, myKeys.secretKey) 

    var response = {pubKey: myKeys.publicKey}
    
    return merge({
      // encrypting the response with the requester's pub key from our contacts database
      // if a requester spoofs his ID he won't be able to read the response anyway
      response: rsaPubKey.encrypt(response, CIPHER_ENC),
    }, cryptoOps(shared))
            
//  } catch (err) {console.log(err); return null} // failed
}

// common part to both client and server
function handleFstMessage(fstMsg, socket) {
  var fstMsg = fstMsg + "" // the "cast"... wtf
  var split = readLenPrefix(fstMsg)
  socket.unshift(split[1])
  return split[0] 
}

// check out my serialization bitch...
function lenPrefix(str) { return numToChars(str.length) + str }

function readLenPrefix(s) { // reads a length prefix stream, returns it and the remainder  
  var lenLen = 2 // you get it right?
  var len = strToNum(s)
  s = s.substring(lenLen) // going past the len prefix
  return [s.substring(0, len), s.substring(len)]
}

function numToChars(n) {
  return String.fromCharCode(Math.floor(n / 256)) + String.fromCharCode(n % 256)
}

function strToNum(s) { return s.charCodeAt(1) + s.charCodeAt(0) * 256}

function cryptoOps(shared) {
  var nonceLen = nacl.secretbox.nonceLength 

  // we are prepending the nonce to the encrypted message in plain
  return {
    encrypt: function (msg) {
        var nonce = nacl.randomBytes(nonceLen)
        return concatAs(nonce, nacl.secretbox(msg, nonce, shared))
    },
    decrypt: function (cipher) {
        var nonce = cipher.subarray(0, nonceLen)
        var encrypted = cipher.subarray(nonceLen)
        return nacl.secretbox.open(encrypted, nonce, shared) 
    }
  }
}

function connWithSocket(socket, conn) {
  return {src: pipe(socket, conn.src), sink: pipe(conn.sink, new SocketStream(socket))}  
}

function conn(cryptoOps) {
  return {
    src: pipe(new jls.JSONParseStream(), mapSSync(incoming(cryptoOps.decrypt))),
    sink: mapSSync(R.compose(jsonChunk, outgoing(cryptoOps.encrypt)))
  }
}

function incoming(decrypt) {return R.compose(JSON.parse, Uint8ToString, decrypt, StringToUint8, head)}
function outgoing(encrypt) {return R.compose(singleton, Uint8ToString, encrypt, StringToUint8, JSON.stringify)}

function jsonChunk(cipher) { return JSON.stringify(cipher) + "\n"}

// EXTRA DHT

//augmenting the dht
function augmentDHT(dht) {
  console.log("augmenting DHT")
  // adding callback notifications for peers on specific infohashes
  var peerCbs = {}

  dht.onInfoHashPeer = function (ih, cb) {
    peerCbs[ih] = cb

  }

  dht.on('peer', function (addr, hash, from) {
    if (peerCbs[hash]) setImmediate(function () {peerCbs[hash](addr, from)})
  })
}

// EXTRA STREAMING

// map stream sync
function mapSSync(f) { return mapS(function(v, cb) {cb(null, f(v))}) }

function concatAs(a1, a2) {
  var big = new Uint8Array(a1.length + a2.length)
  big.set(a1, 0)
  big.set(a2, a1.length)
  return big
}

function StringToUint8(str) {
  var arr=[]
  for(var i=0,j=str.length;i<j;++i) {
    arr.push(str.charCodeAt(i))
  }
  return new Uint8Array(arr)
}


function Uint8ToString(u8a){
  var CHUNK_SZ = 0x8000
  var c = []
  for (var i=0; i < u8a.length; i+=CHUNK_SZ) {
    c.push(String.fromCharCode.apply(null, u8a.subarray(i, i+CHUNK_SZ)))
  }
  return c.join("")
}

// transforms {"0": 1, "1", 2} -> [1, 2]
function objToArray(buf) {
  var len = (R.filter(function(x) {return !isNaN(x)}, Object.keys(buf))).length
  var a = new Array(len)
  for (i = 0; i < len; i++) a[i] = buf[i]
  return a  
}

function printID (x) {console.log(x); return x}

function head(a) {return a[0]}
function singleton(a) {return [a]}
function reverseBuf(buffer) {
  return new Buffer(buffer.toString().split("").reverse().join(""))
}
