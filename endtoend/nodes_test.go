package endtoend

import (
	"fmt"
	"testing"
)

func TestAddNodeStaticToken(t *testing.T) {
	config, err := newConfiguration("add-node-static-token", `
teleport:
  nodename: server01
  data_dir: {{ .DataDir }}
  storage:
    type: dir
    path: {{ .StorageDir }}

auth_service:
  enabled: yes
  cluster_name: "example.com"
  listen_addr: 0.0.0.0:3025
  tokens:
    - node:foo
  authentication:
    type: local
    second_factor: off

proxy_service:
  enabled: yes
  listen_addr: "0.0.0.0:3023"
  tunnel_listen_addr: "0.0.0.0:3024"
  web_listen_addr: "0.0.0.0:3080"
  public_addr: ["proxy.example.com"]
  tunnel_public_addr: ["localhost:3024"]

ssh_service:
   enabled: "yes"
   listen_addr: "0.0.0.0:3022"
`)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("--> %v.\n", config)
	//teleport, err := newTeleport(config)
	//if err != nil {
	//	t.Fatal(err)
	//}

}

func TestAddNodeEphemeralToken(t *testing.T) {
}

func TestAddNodeInvalidToken(t *testing.T) {
}

func TestAddNodeRevokedToken(t *testing.T) {
}
