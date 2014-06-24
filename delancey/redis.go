package main

import (
	"time"

	redigo "github.com/garyburd/redigo/redis"
)

// poolConn retrurns a redis connection to RedisPath.
func poolConn() (redigo.Conn, error) {
	return redigo.Dial("tcp", RedisPath)
}

// Redis contains a connection pool.
type Redis struct {
	pool *redigo.Pool
}

// NewRedis creates a new Redis pool.
func NewRedis() *Redis {
	return &Redis{
		pool: &redigo.Pool{
			MaxIdle:     10,
			IdleTimeout: 240 * time.Second,
			Dial:        poolConn,
		},
	}
}

// Write implements io.Writer writing logs.
func (redis *Redis) Write(b []byte) (int, error) {
	return len(b), redis.WriteLogs(ApplicationID, ServiceName, b)
}

// WriteLogs publishes data to an application.
func (redis *Redis) WriteLogs(appId, serviceName string, data []byte) error {
	conn := redis.pool.Get()
	defer conn.Close()

	return conn.Send("PUBLISH", "logs:"+appId, serviceName+": "+string(data))
}

// UpdateState updates the current state for an applications service.
func (redis *Redis) UpdateState(appId, serviceName, state string) error {
	conn := redis.pool.Get()
	defer conn.Close()

	return conn.Send("PUBLISH", "state:"+appId+":"+serviceName+": "+state)
}

// Close closes the redis pool.
func (redis *Redis) Close() error {
	return redis.pool.Close()
}