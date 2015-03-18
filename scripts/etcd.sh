#!/bin/bash

# this scripts inits proper keys using local fixtures file

# set user certificate authority
etcdctl set /teleport/auth/user/key "{\"Pub\": \"$(base64 -w 0 fixtures/keys/usr_ca/user_ca.pub)\"}"

# add user alex
etcdctl set /teleport/users/alex/keys/k1 "$(cat fixtures/keys/users/alex-cert.pub)"


