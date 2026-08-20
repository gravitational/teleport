package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e2e/runner/fixtures"
)

func TestNodeVariants(t *testing.T) {
	tests := []struct {
		name     string
		plain    bool
		bpf      bool
		want     []string
		wantBPF  []bool
		anyNodes bool
	}{
		{name: "no node fixtures", want: nil, anyNodes: false},
		{name: "plain only", plain: true, want: []string{"docker-node"}, wantBPF: []bool{false}, anyNodes: true},
		{name: "bpf only", bpf: true, want: []string{"docker-node-bpf"}, wantBPF: []bool{true}, anyNodes: true},
		{
			name: "both requested runs two nodes", plain: true, bpf: true,
			want: []string{"docker-node", "docker-node-bpf"}, wantBPF: []bool{false, true}, anyNodes: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plain, bpf := fixtures.SSHNode.Enabled, fixtures.SSHNodeBPF.Enabled
			t.Cleanup(func() { fixtures.SSHNode.Enabled, fixtures.SSHNodeBPF.Enabled = plain, bpf })

			fixtures.SSHNode.Enabled, fixtures.SSHNodeBPF.Enabled = test.plain, test.bpf

			require.Equal(t, test.want, nodeVariantNames())
			require.Equal(t, test.anyNodes, sshNodeEnabled())

			for i, v := range nodeVariants() {
				require.Equal(t, test.wantBPF[i], v.enhancedRecording, "variant %s", v.name)
			}
		})
	}
}

func TestDockerNodeEntrypoint(t *testing.T) {
	plain := (&dockerNode{}).entrypoint()
	require.Equal(t, []string{"teleport", "start", "--insecure", "-c", "/etc/teleport/node.yaml"}, plain)

	bpf := (&dockerNode{enhancedRecording: true}).entrypoint()
	require.Len(t, bpf, 3)
	require.Equal(t, []string{"sh", "-c"}, bpf[:2])
	require.Contains(t, bpf[2], "mount -t tracefs")
	// exec keeps teleport as PID 1 so it still receives the stop signal.
	require.Contains(t, bpf[2], "; exec "+nodeStartCommand)
}
