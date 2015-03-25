
# Crypto notes


## Crypto protocol

Steps:

* Starting assumption: A and B know each other's public keys.

* A sends B a message encrypted with B's public key containing:
  <curve25519 pub key, A's identity>

* B responds with a message encrypted with A's pub key. that contains
  <B's curve25519 pub key>

* they both perform Diffie hellman get a shared secret and start talking

## Links

RSA is still the way to do public key crypto, the only problem is that the keys are big.

http://security.stackexchange.com/questions/54176/any-alternative-available-for-rsa-is-the-web-still-visible-to-nsa
