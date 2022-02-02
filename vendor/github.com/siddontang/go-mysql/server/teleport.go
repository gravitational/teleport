package server

import (
	. "github.com/siddontang/go-mysql/mysql"
)

func (c *Conn) WriteInitialHandshake() error {
	return c.writeInitialHandshake()
}

func (c *Conn) ReadHandshakeResponse() error {
	return c.readHandshakeResponse()
}

func (c *Conn) GetDatabase() string {
	return c.db
}

func (c *Conn) WriteOK(r *Result) error {
	return c.writeOK(r)
}

func (c *Conn) WriteError(e error) error {
	return c.writeError(e)
}
