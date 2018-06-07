package agentconn

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"regexp"

	"github.com/Microsoft/go-winio"
)

// The Cygwin support below was originally based on this implementation:
// https://github.com/abourget/secrets-bridge/blob/master/pkg/agentfwd/agentconn_windows.go

const microsoftOpenSSHAgentPipe = "\\\\.\\pipe\\openssh-ssh-agent"

var windowsFakeSocket = regexp.MustCompile(`!<socket >(\d+) ([A-Fa-f0-9-]+)`)

func dialAgent(socket string) (net.Conn, error) {
	// If the socket path isn't set, or is set to the named pipe associated with
	// Microsoft's native OpenSSH implementation, try connecting with a native
	// Windows named pipe.
	if socket == "" || socket == microsoftOpenSSHAgentPipe {
		return winio.DialPipe(microsoftOpenSSHAgentPipe, nil)
	}

	// If a socket path was specified, fall back to Cygwin's Unix socket emulation
	socketFileData, err := ioutil.ReadFile(socket)
	if err != nil {
		return nil, err
	}

	matches := windowsFakeSocket.FindStringSubmatch(string(socketFileData))
	if matches == nil {
		return nil, fmt.Errorf("couldn't parse SSH_AUTH_SOCK file %s", socket)
	}

	tcpPort := matches[1]
	key := matches[2]

	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%s", tcpPort))
	if err != nil {
		return conn, err
	}

	b := make([]byte, 16)
	fmt.Sscanf(key,
		"%02x%02x%02x%02x-%02x%02x%02x%02x-%02x%02x%02x%02x-%02x%02x%02x%02x",
		&b[3], &b[2], &b[1], &b[0],
		&b[7], &b[6], &b[5], &b[4],
		&b[11], &b[10], &b[9], &b[8],
		&b[15], &b[14], &b[13], &b[12],
	)

	if _, err = conn.Write(b); err != nil {
		return nil, fmt.Errorf("write b: %v", err)
	}

	b2 := make([]byte, 16)
	if _, err = conn.Read(b2); err != nil {
		return nil, fmt.Errorf("read b2: %v", err)
	}

	pidsUids := make([]byte, 12)
	pid := os.Getpid()
	uid := 0
	gid := pid // for cygwin's AF_UNIX -> AF_INET, pid = gid

	binary.LittleEndian.PutUint32(pidsUids, uint32(pid))
	binary.LittleEndian.PutUint32(pidsUids[4:], uint32(uid))
	binary.LittleEndian.PutUint32(pidsUids[8:], uint32(gid))

	if _, err = conn.Write(pidsUids); err != nil {
		return nil, fmt.Errorf("write pid,uid,gid: %v", err)
	}

	b3 := make([]byte, 12)
	if _, err = conn.Read(b3); err != nil {
		return nil, fmt.Errorf("read pid,uid,gid: %v", err)
	}

	return conn, nil
}
