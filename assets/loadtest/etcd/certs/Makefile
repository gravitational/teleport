#!/bin/bash

# output:
#   ca-key.pem      : private key of the trusted CA
#   ca-cert.pem     : self-signed CA cert
#   client-crt.pem  : client cert signed by CA
# 	client-key.pem  : private key of the client
.PHONY:all
all: ca-cert.pem server-cert.pem client-cert.pem

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
	openssl ca -batch \
		-extensions etcd_client \
		-config openssl.cnf \
		-keyfile ca-key.pem \
		-cert ca-cert.pem \
		-out client-cert.pem \
		-infiles client.csr
	@rm -rf *old index* serial* 01.pem 03.pem client.csr

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
	openssl ca -batch \
		-extensions etcd_server \
		-config openssl.cnf \
		-keyfile ca-key.pem \
		-cert ca-cert.pem \
		-out server-cert.pem \
		-infiles server.csr
	@rm -rf *old index* serial* server.csr

# Generates the "root" private key+cert which will become the trusted CA 
# which can sign client certificates
ca-cert.pem:
	openssl req -x509 \
		-extensions v3_ca \
		-config openssl.cnf \
		-new \
		-keyout ca-key.pem \
		-out ca-cert.pem \
		-subj "/C=US/ST=San Francisco/L=SOMA/O=Gravitational/CN=localhost" \
		-days 3650 \
		-nodes

# removes everything
.PHONY:clean
clean:
	rm -rf *pem *csr *crt index* serial
