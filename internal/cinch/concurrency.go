package cinch

import "runtime"

// boundedWorkers returns a sensible worker-pool size for n independent,
// subprocess- or I/O-bound jobs: capped at 8 to avoid spawning excessive
// concurrent git subprocesses or open file descriptors on large repos, never
// more than n, and never less than 1.
func boundedWorkers(n int) int {
	c := runtime.GOMAXPROCS(0)
	if c > 8 {
		c = 8
	}
	if n < c {
		c = n
	}
	if c < 1 {
		c = 1
	}
	return c
}
