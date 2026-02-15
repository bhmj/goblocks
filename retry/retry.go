package retry

import (
	"math"
	"math/rand/v2"
	"time"
)

type Policy struct {
	MaxAttempts int
	Backoff     time.Duration
	Multiplier  float64
	MaxBackoff  time.Duration
	Jitter      time.Duration
	zeroJitter  bool
}

const (
	defaultAttempts   = 5
	defaultBackoff    = 500 * time.Millisecond
	defaultMultiplier = 2
	defaultMaxBackoff = 5 * time.Second
	defaultJitter     = 200 * time.Millisecond
)

func (p *Policy) NoJitter() *Policy {
	p.zeroJitter = true
	return p
}

// Run executes fn using retry policy p. Stops retrying on success or after p.Attempts retries.
// In case fn returns fatal error, Run exits immediately.
// Note: use `policy.NoJitter().Run(...)` to eliminate jitter. Simple `policy := Policy{Jitter: 0}; policy.Run(...)` will result in default jitter.
func (p *Policy) Run(fn func(attempt int) (err error, fatal error)) error {
	if p.MaxAttempts == 0 {
		p.MaxAttempts = defaultAttempts
	}
	if p.Backoff == 0 {
		p.Backoff = defaultBackoff
	}
	if p.Multiplier == 0 {
		p.Multiplier = defaultMultiplier
	}
	if p.MaxBackoff == 0 {
		p.MaxBackoff = defaultMaxBackoff
	}
	if p.Jitter == 0 && !p.zeroJitter {
		p.Jitter = defaultJitter
	}

	attempt := 0
	var err, fatal error
	for {
		attempt++
		err, fatal = fn(attempt)
		if fatal != nil {
			return fatal
		}
		if err == nil {
			return nil
		}
		if attempt < p.MaxAttempts {
			time.Sleep(p.calcSleepTime(attempt))
		} else {
			break
		}
	}
	return err
}

func (p *Policy) calcSleepTime(attempt int) time.Duration {
	jitter := time.Duration(rand.Float64() * float64(p.Jitter)) //nolint:gosec
	sleepTime := time.Duration(float64(p.Backoff)*math.Pow(p.Multiplier, float64(attempt-1))) + jitter
	sleepTime = min(sleepTime, p.MaxBackoff)
	return sleepTime
}
