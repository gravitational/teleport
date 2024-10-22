#!/bin/sh
cd "$( dirname -- "${0}" )" || exit 1

sleep "$( echo "90 * $(od -An -N4 -tu4 /dev/urandom) / 4294967295" | bc -l )"

ssh_opts="-qn -F tbot_destdir_mux/ssh_config -S /run/user/1000/ssh-control/%C -o ControlMaster=auto -o ControlPersist=60s -o Ciphers=^aes128-gcm@openssh.com -l root"

i=0
while [ $i -lt 10000 ] ; do
  i_pretty="$( printf %04d ${i} )"
  # instant
  ssh ${ssh_opts} -tt "${1}" 'echo "OK ${SSH_SESSION_ID} ${SSH_TELEPORT_HOST_UUID} '"${i_pretty}"' A $( /busybox/date -u +"%Y-%m-%dT%H:%M:%S %s" )"; exit 0' || ( echo ERR A "${1}" >&2 && sleep 1 )
  # line every 5 seconds, 120 sec
  ssh ${ssh_opts} -T "${1}" 'j=0; while [ $j -lt 24 ]; do echo "OK ${SSH_SESSION_ID} ${SSH_TELEPORT_HOST_UUID} '"${i_pretty}"' B $( printf %04d $j ) $( /busybox/date -u +"%Y-%m-%dT%H:%M:%S %s" )"; sleep 5; j=$(( j + 1 )); done; exit 0' || ( echo ERR B "${1}" >&2 && sleep 1 )
  # data transfer and no wait
  ssh ${ssh_opts} -T "${1}" '/busybox/dd if=/dev/zero bs=4096 count=256 2>/dev/null; echo "OK ${SSH_SESSION_ID} ${SSH_TELEPORT_HOST_UUID} '"${i_pretty}"' C $( /busybox/date -u +"%Y-%m-%dT%H:%M:%S %s" )"; exit 0' | tr -d '\000' || ( echo ERR C "${1}" >&2 && sleep 1 )
  # line every 5 seconds, 240 sec
  ssh ${ssh_opts} -T "${1}" 'j=0; while [ $j -lt 48 ]; do echo "OK ${SSH_SESSION_ID} ${SSH_TELEPORT_HOST_UUID} '"${i_pretty}"' D $( printf %04d $j ) $( /busybox/date -u +"%Y-%m-%dT%H:%M:%S %s" )"; sleep 5; j=$(( j + 1 )); done; exit 0' || ( echo ERR D "${1}" >&2 && sleep 1 )

  i=$(( i + 1 ))
done

exit 0
