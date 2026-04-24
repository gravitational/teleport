package ssh

import "io"

type Client struct{}

func (*Client) NewSession() (*Session, error)

type Session struct{}

func (*Session) StdinPipe() (io.WriteCloser, error)
func (*Session) StdoutPipe() (io.Reader, error)
func (*Session) StderrPipe() (io.Reader, error)

func (*Session) RequestSubsystem(string) error

func (*Session) Wait() error
