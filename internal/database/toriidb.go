package database

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pardnchiu/ToriiDB/core/store"
)

type toriiBackend struct {
	store *store.Store
}

type scriptMeta struct {
	Path     string  `json:"path"`
	Language string  `json:"language"`
	Latest   int64   `json:"latest"`
	Versions []int64 `json:"versions"`
}

func newToriiBackend() (*toriiBackend, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	dir := filepath.Join(homeDir, ".config", "pardnchiu", "hakorun")

	s, err := store.New(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to init store: %w", err)
	}

	return &toriiBackend{store: s}, nil
}

func (b *toriiBackend) Close() error {
	return b.store.Close()
}

func (b *toriiBackend) Add(ctx context.Context, script Script) (int64, error) {
	hash := md5.Sum([]byte(script.Path))
	hashStr := hex.EncodeToString(hash[:])
	timestamp := time.Now().Unix()
	metaKey := fmt.Sprintf("meta:%s", hashStr)
	codeKey := fmt.Sprintf("code:%s:%d", hashStr, timestamp)

	m := scriptMeta{}
	if entry, ok := b.store.Get(metaKey); ok {
		if err := json.Unmarshal([]byte(entry.Value()), &m); err != nil {
			return 0, fmt.Errorf("failed to decode meta: %w", err)
		}
	}
	m.Path = script.Path
	m.Language = script.Language
	m.Latest = timestamp
	m.Versions = append(m.Versions, timestamp)

	metaRaw, err := json.Marshal(m)
	if err != nil {
		return 0, fmt.Errorf("failed to encode meta: %w", err)
	}

	// * write code before meta so meta never points to a missing code version
	if err := b.store.Set(codeKey, script.Code, store.SetDefault, nil); err != nil {
		return 0, fmt.Errorf("failed to save code: %w", err)
	}
	if err := b.store.Set(metaKey, string(metaRaw), store.SetDefault, nil); err != nil {
		return 0, fmt.Errorf("failed to update meta: %w", err)
	}

	return timestamp, nil
}

func (b *toriiBackend) Get(ctx context.Context, path string, version int64) (*Script, error) {
	hash := md5.Sum([]byte(path))
	hashStr := hex.EncodeToString(hash[:])
	metaKey := fmt.Sprintf("meta:%s", hashStr)

	entry, ok := b.store.Get(metaKey)
	if !ok {
		return nil, fmt.Errorf("script not found")
	}

	var m scriptMeta
	if err := json.Unmarshal([]byte(entry.Value()), &m); err != nil {
		return nil, fmt.Errorf("failed to get meta: %w", err)
	}

	if m.Language == "" {
		return nil, fmt.Errorf("language not found in meta")
	}
	if version == 0 {
		version = m.Latest
	}

	codeKey := fmt.Sprintf("code:%s:%d", hashStr, version)
	codeEntry, ok := b.store.Get(codeKey)
	if !ok {
		return nil, fmt.Errorf("assign version not found")
	}

	return &Script{
		Path:      m.Path,
		Code:      codeEntry.Value(),
		Language:  m.Language,
		Timestamp: version,
	}, nil
}
