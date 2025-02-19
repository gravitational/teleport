#!/bin/bash

set -euo pipefail

image_tag="18.0.0-dev.forrest-pdp.2"
prod_node_id="3c6924b0-3889-48d5-a886-ae79321dbeab"
staging_node_id="f48b739e-7b6c-47e5-997e-f52e43273fae"

echo "starting teleport container ($image_tag)..." >&2

container_id="$(docker run --detach --volume "$(dirname "$0")/example-config:/etc/teleport" "public.ecr.aws/gravitational-staging/teleport-distroless-debug:$image_tag" --bootstrap=/etc/teleport/bootstrap.yaml)"

echo "successfully started container id=$container_id" >&2

echo "waiting for teleport to init..." >&2

sleep 5 # 5s is arbitrary, but seems to work pretty reliably

echo "running example queries..." >&2

echo "[q1] check if alice has access to prod node (should be denied)" >&2

docker exec -it "$container_id" tctl --config=/etc/teleport/teleport.yaml decision evaluate-ssh-access --login=alice --username=alice --server-id="$prod_node_id"

echo "[q2] check if alice has access to staging node (should be allowed)" >&2

docker exec -it "$container_id" tctl --config=/etc/teleport/teleport.yaml decision evaluate-ssh-access --login=alice --username=alice --server-id="$staging_node_id"

echo "[q3] check if root has access to staging node (should be denied)" >&2

docker exec -it "$container_id" tctl --config=/etc/teleport/teleport.yaml decision evaluate-ssh-access --login=root --username=alice --server-id="$staging_node_id"

echo "halting container..." >&2

docker rm -f "$container_id"
