set -e
set -x

make -f /usr/share/selinux/devel/Makefile teleport.pp
semodule -r teleport || true
semodule -i teleport.pp

restorecon -rv /usr/local/bin/
restorecon -rv /run/
restorecon -rv /etc/
restorecon -rv /var/lib/teleport/

# uncomment to clear audit logs, making it easier to find relevant SELinux policy deny logsk
#truncate -s 0 /var/log/audit/audit.log
