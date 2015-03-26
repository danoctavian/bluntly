# bluntly

talk to whoever, wherever safely with no bullshit

Bluntly allows you to setup a secure connection to a peer only by knowing its public key (and it knowing yours).

No servers needed, no NAT getting in the way. It's a POC (proof-of-concept) so don't start talking with snowden with this shit.

![alt text](https://github.com/danoctavian/bluntly/blob/master/docs/chat-diagram.png "fuuak")

Disclaimer: there's no really clever ideas here, i'm just stiching some things together. It's a hacky implementation (I can't justify
myself for this, it just felt good to screw around in javascript).

How?

* exchange pub keys with your partner (i don't know how, figure it out)

* client looks up listener in bittorrent DHT by his pubkey (Lpk) using info\_hash = sha1(Lpk)

* once you it has its IP, connect (if there's a NAT in the way, just penetrate it. see how below)

* client sends encrypted handshake (with RSA pub key) to listener containing your curve25519 pub key 

* listener responds with its curve25519 pub key in a handshake response encrypted with 
your RSA pubkey

* both do Diffie hellman and derive shared secret. encrypt all messages from here on using that.


### RUN IT

Install dependencies.

* nodejs https://nodejs.org/download/

* npm 

Get code dependencies:

```bash
cd node-bluntly
npm install 
``` 

for a quick run, use the RSA key pair checked in the repo for both parties. Get 2 machines and do

Go to the test-data directory:

```bash
cd test-data
```

For the server:

```bash
node ../index.js -s 5678 # or whatever ports you want to listen on
``` 

For the client:

```bash
node ../index.js -c myself
``` 

Obviously the above is not secure, because the private key is in the open.

### RUNTIME REQUIREMENTS

bluntly expects you provide with a config file containing:

```javascript
{
  "dhtPort": 20000, // the port 
  "ownKey": {"pub": "mypub.rsa", "priv": "mypriv.rsa"}, // filepaths to your pub and private rsa keys
  "ID": "myself", // your bluntly name
  "contactsDir": "contacts", // the path of the directory containing your friends' pub keys
  // if you're holepunching through NAT, the port you want other peers to UDP connect to
  // if it's not specified, hole punch will not be attempted at all and it will fail if 
  // no direct TCP connection is possible
  "holePunch": {"recvPort": 23456}  
}
```

If you don't explicitly specify a path to a config file with --config=myconfigfile it will look for a 
file *blunt-conf.json* in the current directory and use that as a config file.

**contactsDir**  contains a json file called *index.json* which specifies a mapping from friend ids to the file containing
their pub key in the contactsDir directory (it's a relative path) as such

```javascript
{
  "yourmom": "mom.rsa",
  "yourdad": "dad.rsa",
  "myself": "myself.rsa"
}
```

A good example is the test-data directory, containing a conf file and a contacts dir.

### NAT penetration

Client notifies listener of interest to connect by announcing itself to rendezvous\_info\_hash =  reverse(sha1(Spk)) (the reverse function is kind of arbitrary)

Listener constantly polls the bittorrent DHT for peers announced rendezvous\_info\_hash . Learns about the intention of a peer to connect.

They both know each other's IPs and proceed to do UDP NAT penetration as done by [chownat]( http://samy.pl/chownat/).

once the hole punch succeeds, the 2 parties switch to UTP protocol running over UDP for reliability.

What would be nice: not have the rendezvous info hash and use the technique presented used by [pwnat](http://samy.pl/pwnat/)

###  Motivation

Just wanted smth to easily use without making accounts here and there and having end to end encryption.

I wanted to hack something in javascript to see how it's like to build prototypes with it.

