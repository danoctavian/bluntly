var dgram = require("dgram")
var utp = require("./utp")

// CONSTANTS

var PUNCH = "P"
var ACK = "A"
var PUNCH_INTERVAL = 500

exports.udpHolePunch = udpHolePunch

function udpHolePunch(socket, target, cb, timeout) {
  timeout = timeout || 10000  // in millis

  var client = {receivedAck: false, outMsg: PUNCH, cancelled: false, done: false}
  // keep sending punches till you get an ack
  sendPunch(client, function() { send(socket, target, client.outMsg) })
  function onData(data, src) {

    // yeah i actually mean != here so it casts all those funny formats
    if (src.address != target.host || src.port != target.port) {
      return // we aren't interested in this msg
    }
    if (data == ACK && !client.receivedAck) {
      client.receivedAck = true //we're done
      client.ackTimestamp = now()
      client.outMsg = ACK
      client.done = true
      // remove the listener and call cb
      socket.removeListener('message', onData)
      setImmediate(cb)
    } else if (data == PUNCH) { // got a punch, start acking
      client.outMsg = ACK
    }
  }

  setTimeout(function() {
    if (client.done) return
    client.cancelled = true // stop attempting to hole punch
    setImmediate(cb, new Error("hole punch timeout"))
  }, timeout)

  socket.on('message', onData)
}

function sendPunch(client, send) {
  // hacky way of getting the ack on the other side before you stop
  // may NOT always work in case all packets are lost.
  if ((client.receivedAck && now() - client.ackTimestamp >= PUNCH_INTERVAL * 4)
     || client.cancelled) return 
  send() 
  setTimeout(function() {sendPunch(client, send)}, PUNCH_INTERVAL)
}

function send(socket, conn, msg, cb) {
  //console.log("sending msg " + msg + " to " + JSON.stringify(conn))
  socket.send(msg, 0, msg.length, conn.port, conn.host, function(err, bytes) {
    if (err) {
      udp_in.close()
      console.log('# stopped due to error: %s', err)
    } else {
      if (cb) cb()
    }
  })
}

function now() {return new Date().getTime()}
