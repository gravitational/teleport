package common

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
)

func Upgrade(proxyAddr string) (*tls.Conn, error) {
	// TODO detect if proxy is behind ALB
	// TODO handle insecure
	conn, err := tls.Dial("tcp", proxyAddr, &tls.Config{})
	if err != nil {
		return nil, trace.Wrap(err)

	}
	u := url.URL{
		Host:   proxyAddr,
		Scheme: "https",
		Path:   fmt.Sprintf("/webapi/connectionupgrate"),
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Add("Upgrade", "custom")
	req.Header.Add("Connection", "upgrade")

	if err = req.Write(conn); err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, trace.BadParameter("failed to switch Protocols %v", resp.StatusCode)
	}

	return conn, nil
}
