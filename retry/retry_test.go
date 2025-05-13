package retry

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetry(t *testing.T) {
	a := assert.New(t)

	zeroJitterPolicy := &Policy{MaxAttempts: 3}
	zeroJitterPolicy = zeroJitterPolicy.NoJitter()
	testCases := []struct {
		// in
		TestPolicy *Policy
		// out
		ExpectedTotalTime float64 // ms
	}{
		// default values
		{&Policy{}, 0 + 500 + 500*2 + 500*2*2 + 500*2*2*2},
		// default values, with attempts
		{&Policy{MaxAttempts: 2}, 0 + 500},
		// zero jitter
		{zeroJitterPolicy, 0 + 500 + 500*2},
		// full set of params
		{
			&Policy{MaxAttempts: 8, Backoff: 20 * time.Millisecond, Multiplier: 1.8, MaxBackoff: 100 * time.Millisecond, Jitter: time.Millisecond},
			0 + 20 + 20*1.8 + 20*1.8*1.8 + 20*1.8*1.8*1.8 + 100 + 100 + 100,
		},
		// "reverse" backoff
		{&Policy{Backoff: 100 * time.Millisecond, Multiplier: 0.5, Jitter: time.Millisecond}, 0 + 100 + 100/2 + 100/2/2 + 100/2/2/2},
	}
	for i, tst := range testCases {
		t.Logf("retry: test case %d", i)
		sleepTimes := make([]time.Duration, 10) // sleep time stats
		policy := tst.TestPolicy
		err := errors.New("normal error")
		prev := time.Now()
		var lastAttempt int
		_ = policy.Run(func(attempt int) (error, error) {
			slept := time.Since(prev)
			t.Logf("%v\n", slept)
			prev = time.Now()
			sleepTimes[attempt] = slept
			lastAttempt = attempt
			return err, nil
		})
		a.Equal(tst.TestPolicy.MaxAttempts, lastAttempt)
		expectedDuration := time.Duration(tst.ExpectedTotalTime) * time.Millisecond
		delta := sumDurations(sleepTimes) - expectedDuration
		maxJitterPerRun := tst.TestPolicy.Jitter * time.Duration(tst.TestPolicy.MaxAttempts-1) // first attempt has no delay/jitter
		a.Less(delta.Abs(), maxJitterPerRun+time.Millisecond*10)                               // permissible delta
	}

	// fatal error terminates retry loop immediately
	lastAttempt := 0
	policy := Policy{MaxAttempts: 100}
	err := policy.Run(func(attempt int) (error, error) {
		lastAttempt = attempt
		if attempt > 1 {
			return nil, errors.New("fatal") // simulate fatal error on 2nd attempt
		}
		return errors.New("dummy"), nil
	})
	a.Equal(err.Error(), "fatal")
	a.Equal(2, lastAttempt)
}

func sumDurations(durations []time.Duration) time.Duration {
	var sum time.Duration
	for _, dur := range durations {
		sum += dur
	}
	return sum
}
