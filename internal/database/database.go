package database

import (
	"context"
	"fmt"
)

type Script struct {
	Path      string
	Code      string
	Language  string
	Timestamp int64
}

type backend interface {
	Add(ctx context.Context, script Script) (int64, error)
	Get(ctx context.Context, path string, version int64) (*Script, error)
	Close() error
}

var DB backend

func Init() error {
	b, err := newBackend()
	if err != nil {
		return fmt.Errorf("failed to init backend: %w", err)
	}

	DB = b
	return nil
}

func Close() error {
	return DB.Close()
}
