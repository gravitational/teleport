#!/bin/bash

# this script generates deployment, configs, and entrypoint for running
# multiple teleport nodes inside of a single container.  used for scaling
# tests that involve more nodes than there are available IPs for a given
# kubernetes cluster.

TELEPORT_IMAGE=${TELEPORT_IMAGE:-'quay.io/gravitational/teleport-ent:9.0.0'}
TELEPORT_FLAGS=${TELEPORT_FLAGS:-'--debug --insecure'}
CFG_OUT=${CFG_OUT:-multinode-cfg-gen}
NODE_COUNT=${NODE_COUNT:-1}
NODE_SSH_PORT=3022

# pull in NODE_TOKEN
source ./secrets/secrets.env

# ensure that NODE_TOKEN is now present.
NODE_TOKEN=${NODE_TOKEN:?}

# create ouput dir if does not exist
mkdir -p $CFG_OUT

# creates a node config file
mk_config () {
    cat <<EOF
teleport:
  data_dir: /var/lib/teleport-${i:?}
  log:
    severity: DEBUG
  storage:
    type: dir
  auth_servers: ["auth:3025"]
  auth_token: "node-${NODE_TOKEN:?}"
auth_service:
  enabled: false
proxy_service:
  enabled: false
ssh_service:
  enabled: true
  listen_addr: 0.0.0.0:${ssh_port:?}
EOF
}

# creates a port declarataion for a specific node
mk_port () {
cat <<EOF
            - name: nodessh-${i:?}
              containerPort: ${ssh_port:?}
              protocol: TCP
EOF
}


# creates a deployment file
mk_deployment() {
    cat <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    teleport-role: node
  name: node
  namespace: loadtest
spec:
  replicas: 1
  selector:
    matchLabels:
      teleport-role: node
      node: regular
  template:
    metadata:
      labels:
        teleport-role: node
        node: regular
    spec:
      containers:
        - image: ${TELEPORT_IMAGE:?}
          name: teleport
          command: ["/etc/teleport/entrypoint.sh"]
          args: ["-d", "--insecure"]
          ports: ${ports:?}
          volumeMounts:
            - mountPath: /etc/teleport
              name: config
              readOnly: true
      volumes:
        - configMap:
            name: node-config
            defaultMode: 0744
          name: config
EOF
}


# creates the entrypoing script
mk_entrypoint() {
    cat <<EOF
#!/bin/bash

${cmds:?}

wait
EOF
}

# aggregator for deployment port specs
ports=""

# aggregator for entrypoint commands
cmds=""

# main loop that creates config files and updates aggregator variables
for (( i=0; i<$NODE_COUNT; i++))
do
    let ssh_port=$NODE_SSH_PORT+$i*100
    mk_config > $CFG_OUT/teleport-${i}.yaml
    ports=$(printf "${ports}\n$(mk_port)")
    cmds=$(printf "${cmds}\nteleport start \${@} --config=/etc/teleport/teleport-${i}.yaml &")
done

# write the entrypoint script
mk_entrypoint > $CFG_OUT/entrypoint.sh

# write the deployment
mk_deployment > node-deployment-gen.yaml
