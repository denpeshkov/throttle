package throttletest

import (
	"context"
	"testing"

	rds "github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

type rediser struct {
	rdb *rds.Client
}

func (r rediser) ScriptLoad(ctx context.Context, script string) (string, error) {
	return r.rdb.ScriptLoad(ctx, script).Result()
}
func (r rediser) EvalSHA(ctx context.Context, sha1 string, keys []string, args ...any) (any, error) {
	return r.rdb.EvalSha(ctx, sha1, keys, args...).Result()
}
func (r rediser) Del(ctx context.Context, keys ...string) (int64, error) {
	return r.rdb.Del(ctx, keys...).Result()
}

func setupRedis(tb testing.TB) rediser {
	tb.Helper()

	reds, err := redis.Run(context.Background(), "redis:latest", testcontainers.WithLogger(testcontainers.TestLogger(tb)))
	if err != nil {
		tb.Fatalf("Failed to create Redis container %v", err)
	}
	addr, err := reds.Endpoint(context.Background(), "")
	if err != nil {
		tb.Fatalf("Failed to get Redis container endpoint %v", err)
	}
	rdb := rds.NewClient(&rds.Options{Addr: addr})
	tb.Cleanup(func() {
		if err := reds.Terminate(context.Background()); err != nil {
			tb.Fatalf("Failed to terminate Redis container %v", err)
		}
	})
	return rediser{rdb: rdb}
}
