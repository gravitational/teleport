#!/bin/bash

syslogd&
/usr/sbin/sshd -D
