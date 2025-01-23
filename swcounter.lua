-- this script has side-effects, so it requires replicate commands mode (deprecated as of Redis v7.0)
redis.replicate_commands()

-- key is a key to associate with this rate limiter
-- prev_window is the timestamp of the start of the previous window
-- cur_window is the timestamp of the start of the current window
-- limit_events is the maximum number of events to be allowed in a limiting interval. It must be > 0
-- limit_interval is the duration of the limiting interval in milliseconds
-- now is the current Unix time in milliseconds
-- key_ttl is an optional timeout in milliseconds for the key. It must be >= than limit_interval
-- returns limited, remaining, delay
local key = KEYS[1]
local prev_window = KEYS[2]
local cur_window = KEYS[3]
local limit_events = tonumber(ARGV[1])
local limit_interval = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local key_ttl = tonumber(ARGV[4])

local vals = redis.call('HMGET', key, prev_window, cur_window)
local prev_cnt = tonumber(vals[1]) or 0
local cur_cnt = tonumber(vals[2]) or 0

local limited = 1
local prev_cover = limit_interval - (now % limit_interval)
local count = math.floor(prev_cover / limit_interval * prev_cnt + cur_cnt)

if count < limit_events then
    redis.call('HINCRBY', key, cur_window, 1)
    redis.call('HPEXPIRE', key, key_ttl, 'NX', 'FIELDS', 1, cur_window)
    limited = 0
    if count + 1 < limit_events then -- not the last event
        return { 0, limit_events - count - 1, 0 }
    end
end

local delay
if prev_cnt == 0 then
    delay = prev_cover
else
    delay = prev_cover % (limit_interval / prev_cnt)
end
return { limited, 0, delay }
