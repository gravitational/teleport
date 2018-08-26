import base64
from subprocess import call

template = '{"kind":"node","version":"v2","metadata":{"name":"%s","labels":{"group":"gravitational/devc"}},"spec":{"addr":"%s","hostname":"%s","cmd_labels":{"kernel":{"period":"5m0s","command":["/bin/uname","-r"],"result":"4.15.0-32-generic"}},"rotation":{"current_id":"","started":"0001-01-01T00:00:00Z","grace_period":"0s","last_rotated":"0001-01-01T00:00:00Z","schedule":{"update_clients":"0001-01-01T00:00:00Z","update_servers":"0001-01-01T00:00:00Z","standby":"0001-01-01T00:00:00Z"}}}}'

for i in range(1, 4000):
    node = template % ("node-%d" % i, "127.0.0.%d:3022" %i, "planet-%d"%i)
    call(['./etcdctl.sh', 'set', '/teleport/namespaces/default/nodes/node-%d'%i, base64.b64encode(node)])
    print "set node %i" % i
    


