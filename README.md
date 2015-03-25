# blunt-speak

talk to whoever safely with no bullshit

No servers, no motherfucking NAT getting in the way. It's a POC (proof-of-concept) so don't start talking with snowden with this shit.

![alt text](https://github.com/danoctavian/bluntly/blob/master/docs/chat-diagram.png "fuuak")

Disclaimer: there's no really clvere ideas here, i'm just stiching some things together.

How?

* exchange pub keys with your partner (i don't fucking know how, figure it out)

* client looks up listener in bittorrent DHT by his pubkey (Lpk) using info\_hash = sha1(Lpk)

* once you it has its IP, connect (if there's a NAT in the way, just penetrate it. see how below)

* client sends encrypted handshake (with RSA pub key) to listener containing your curve25519 pub key 

* listener responds with its curve25519 pub key in a handshake response encrypted with 
your RSA pubkey

* both do Diffie hellman and derive shared secret. encrypt all messages from here on using that.

### NAT penetration

Client notifies listener of interest to connect by announcing itself to rendezvous\_info\_hash =  reverse(sha1(Spk)) (the reverse function is kind of arbitrary)

Listener constantly polls the bittorrent DHT for peers announced rendezvous\_info\_hash . Learns about the intention of a peer to connect.

They both know each other's IPs and proceed to do UDP NAT penetration as done by [chownat]( http://samy.pl/chownat/).

once the hole punch succeeds, the 2 parties switch to UTP protocol running over UDP for reliability.

What would be nice: not have the rendezvous info hash and use the technique presented used by [pwnat](http://samy.pl/pwnat/)
