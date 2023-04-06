package tbotv2

import "context"

type Store interface {
	Write(ctx context.Context, name string, data []byte) error
	Read(ctx context.Context, name string) ([]byte, error)
	String() string
}
