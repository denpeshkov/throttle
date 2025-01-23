-- this script has side-effects, so it requires replicate commands mode (deprecated as of Redis v7.0)
redis.replicate_commands()

-- key is a key to associate with this rate limiter
-- limit_events is the maximum number of events to be allowed in a limiting interval. It must be > 0
-- limit_interval is the duration of the limiting interval in milliseconds
-- burst is the maximum burst capacity
-- now is the current Unix time in milliseconds
-- n is the number of events to be processed
-- returns limited, remaining, delay
local key = KEYS[1]
local limit_events = tonumber(ARGV[1])
local limit_interval = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local key_ttl = tonumber(ARGV[4])
local burst = tonumber(ARGV[5])
local n = tonumber(ARGV[6])

local vals = redis.call('HMGET', key, 'last', 'tokens')

local last = tonumber(vals[1]) or now
local tokens = tonumber(vals[2]) or burst

if now < last then
    last = now
end

-- Calculate the new number of tokens, due to time that passed.
local elapsed = now - last
local delta = elapsed / limit_interval * limit_events
tokens = tokens + delta
if tokens > burst then
    tokens = burst
end

if n > burst then
    return { 1, tokens, -1 } -- -1 indicates an INF duration
end

local rem_tokens = tokens - n
local delay = 0
local limited = 0
if rem_tokens == 0 then
    delay = n / limit_events * limit_interval
end
if rem_tokens < 0 then
    delay = -rem_tokens / limit_events * limit_interval
    limited = 1
    rem_tokens = tokens
end

if limited == 0 then
    redis.call('HSET', key, 'last', now, 'tokens', rem_tokens)
    redis.call('HPEXPIRE', key, key_ttl, 'FIELDS', 2, 'last', 'tokens')
end

return { limited, rem_tokens, delay }
