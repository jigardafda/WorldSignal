package httpapi

import "time"

// SetStreamPollFallbackForTest overrides the stream heartbeat/poll interval and
// returns the previous value, so tests can exercise the heartbeat path quickly.
func SetStreamPollFallbackForTest(d time.Duration) time.Duration {
	old := streamPollFallback
	streamPollFallback = d
	return old
}
