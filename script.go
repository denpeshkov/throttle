package throttle

import (
	"context"
	"strings"
)

type script struct {
	rds interface {
		// ScriptLoad preloads a Lua script into Redis and returns its SHA-1 hash.
		ScriptLoad(ctx context.Context, script string) (string, error)
		// EvalSHA executes a preloaded Lua script using its SHA-1 hash.
		EvalSHA(ctx context.Context, sha1 string, keys []string, args ...any) (any, error)
	}
	script string
	sha    string
}

func (s script) exec(ctx context.Context, keys []string, args ...any) (any, error) {
	v, err := s.rds.EvalSHA(ctx, s.sha, keys, args...)
	if err != nil && strings.HasPrefix(err.Error(), "NOSCRIPT") {
		if _, err := s.rds.ScriptLoad(ctx, s.script); err != nil {
			return nil, err
		}
		v, err = s.rds.EvalSHA(ctx, s.sha, keys, args...)
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}
