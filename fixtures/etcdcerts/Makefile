#!/bin/bash

# output:
#   ca-key.pem      : private key of the trusted CA
#   ca-cert.pem     : self-signed CA cert
#   client-crt.pem  : client cert signed by CA
# 	client-key.pem  : private key of the client
.PHONY:all
all: clean ca-cert.pem server-cert.pem client-cert.pem
	@rm -rf *csr

# Creates client certificate signed by the signing CA
client-cert.pem:
	# generate client key:
	openssl req \
		-new \
		-nodes \
		-keyout client-key.pem \
		-subj "/C=US/ST=San Francisco/L=SOMA/O=Gravitational/CN=localhost" \
		-out client.csr
	# sign it with CA:
	@touch index.txt
	@echo '03' > serial
	openssl ca -extensions etcd_client \
		-config openssl.cnf \
	    -keyfile signing-ca-key.pem \
	    -cert signing-ca-cert.pem \
	    -out client-cert.pem \
		-infiles client.csr
	@rm -rf *old index* serial* 01.pem 03.pem

# Creates server certificate signed by the signing CA
server-cert.pem:
	# generate server key:
	openssl req \
		-new \
		-nodes \
		-keyout server-key.pem \
		-subj "/C=US/ST=San Francisco/L=SOMA/O=Gravitational/CN=localhost" \
		-out server.csr
	# sign it with CA:
	@touch index.txt
	@echo '01' > serial
	openssl ca -extensions etcd_server \
		-config openssl.cnf \
	    -keyfile signing-ca-key.pem \
	    -cert signing-ca-cert.pem \
	    -out server-cert.pem \
		-infiles server.csr
	@rm -rf *old index* serial*

# Creates trusted CA bundle containing root and signing CAs
ca-cert.pem: signing-ca-cert.pem
	cat root-ca-cert.pem signing-ca-cert.pem > ca-cert.pem

# Generates root CA
root-ca-cert.pem:
	openssl req -x509 \
		-extensions v3_ca \
		-config ./openssl.cnf \
		-new \
		-keyout root-ca-key.pem \
		-out root-ca-cert.pem \
		-subj "/C=US/ST=San Francisco/L=SOMA/O=Gravitational/CN=localhost" \
		-days 3650 \
		-nodes

# Generates signing CA that's signed by the root CA
signing-ca-cert.pem: root-ca-cert.pem
	openssl req \
		-new \
		-nodes \
		-keyout signing-ca-key.pem \
		-subj "/C=US/ST=San Francisco/L=SOMA/O=Gravitational/CN=localhost" \
		-out signing-ca.csr
	@touch index.txt
	@echo '01' > serial
	openssl ca -extensions v3_ca \
		-config ./openssl.cnf \
		-config openssl.cnf \
	    -keyfile root-ca-key.pem \
	    -cert root-ca-cert.pem \
	    -out signing-ca-cert.pem \
		-infiles signing-ca.csr
	@rm -rf *old index* serial*

# removes everything
.PHONY:clean
clean:
	rm -rf *pem *csr *crt index* serial
