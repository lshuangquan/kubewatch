package redis

import (
	"encoding/json"
	"errors"
	"kubewatch/config"
	kbEvent "kubewatch/pkg/event"
	"log"
	neturl "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
)

var (
	defaultConfig = &config.RedisPool{
		MaxIdle:                10,
		MaxActive:              500,
		IdleTimeout:            300,
		Wait:                   true,
		ReadTimeout:            15,
		WriteTimeout:           15,
		ConnectTimeout:         15,
		NormalTasksPollPeriod:  1000,
		DelayedTasksPollPeriod: 20,
	}
)

type Redis struct {
	Url      string
	KeKey    string
	pool     *redis.Pool
	host     string
	password string
	db       int
}

// Init prepares Redis configuration
func (r *Redis) Init(c *config.Config) error {
	var err error
	r.Url = c.Handler.Redis.Url
	r.KeKey = c.Handler.Redis.KeKey
	r.host, r.password, r.db, err = parseRedisURL(c.Handler.Redis.Url)
	r.pool = &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 300 * time.Second,
		MaxActive:   500,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", r.host, redis.DialPassword(r.password), redis.DialDatabase(r.db))
			if err != nil {
				return nil, err
			}
			return c, nil
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}
	log.Printf("new redis pool at %s", r.host)
	return err
}

// ObjectCreated calls notifyRedis on event creation
func (r *Redis) ObjectCreated(obj interface{}) {
	notifyRedis(r.pool, r.KeKey, obj, "created")
}

// ObjectDeleted calls notifyRedis on event creation
func (r *Redis) ObjectDeleted(obj interface{}) {
	notifyRedis(r.pool, r.KeKey, obj, "deleted")
}

// ObjectUpdated calls notifyRedis on event creation
func (r *Redis) ObjectUpdated(oldObj, newObj interface{}) {
	notifyRedis(r.pool, r.KeKey, newObj, "updated")
}

// TestHandler tests the handler configurarion by sending test messages.
func (r *Redis) TestHandler() {
}

func notifyRedis(pool *redis.Pool, key string, obj interface{}, action string) {
	e := kbEvent.New(obj, action)
	msg, err := json.Marshal(e)
	if err != nil {
		log.Printf("Marshal event error: %v", err)
	}
	conn := pool.Get()
	defer conn.Close()
	_, err = redis.Int(conn.Do("PUBLISH", key, msg))
	if err != nil {
		log.Fatalf("redis publish %s %s, err: %v", key, msg, err)
	}
}

// ParseRedisURL ...
func parseRedisURL(url string) (host, password string, db int, err error) {
	// redis://pwd@host/db
	var u *neturl.URL
	u, err = neturl.Parse(url)
	if err != nil {
		return
	}
	if u.Scheme != "redis" {
		err = errors.New("No redis scheme found")
		return
	}
	if u.User != nil {
		var exists bool
		password, exists = u.User.Password()
		if !exists {
			password = u.User.Username()
		}
	}

	host = u.Host

	parts := strings.Split(u.Path, "/")
	if len(parts) == 1 {
		db = 0 //default redis db
	} else {
		db, err = strconv.Atoi(parts[1])
		if err != nil {
			db, err = 0, nil //ignore err here
		}
	}

	return
}
