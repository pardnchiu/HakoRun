//go:build redis

package database

func newBackend() (backend, error) {
	return newRedisBackend()
}
