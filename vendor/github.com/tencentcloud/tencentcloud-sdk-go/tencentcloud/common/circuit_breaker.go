// a simple golang breaker according to https://docs.microsoft.com/en-us/previous-versions/msp-n-p/dn589784(v=pandp.10)

package common

import (
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	defaultBackupEndpoint    = "ap-guangzhou.tencentcloudapi.com"
	defaultMaxFailNum        = 5
	defaultMaxFailPercentage = 75
	defaultWindowLength      = 1 * 60 * time.Second
	defaultTimeout           = 60 * time.Second
)

var (
	// ErrOpenState is returned when the CB state is open
	errOpenState = errors.New("circuit breaker is open")
)

// counter use atomic operations to ensure consistency
// Atomic operations perform better than mutex
type counter struct {
	failures             int
	all                  int
	consecutiveSuccesses int
	consecutiveFailures  int
}

func newRegionCounter() counter {
	return counter{
		failures:             0,
		all:                  0,
		consecutiveSuccesses: 0,
		consecutiveFailures:  0,
	}
}

func (c *counter) onSuccess() {
	c.all++
	c.consecutiveSuccesses++
	c.consecutiveFailures = 0
}

func (c *counter) onFailure() {
	c.all++
	c.failures++
	c.consecutiveSuccesses = 0
	c.consecutiveSuccesses = 0
}

func (c *counter) clear() {
	c.all = 0
	c.failures = 0
	c.consecutiveSuccesses = 0
}

// State is a type that represents a state of CircuitBreaker.
type state int

// These constants are states of CircuitBreaker.
const (
	StateClosed state = iota
	StateHalfOpen
	StateOpen
)

type breakerSetting struct {
	// backupEndpoint
	// the default is "ap-guangzhou.tencentcloudapi.com"
	backupEndpoint string
	// max fail nums
	// the default is 5
	maxFailNum int
	// max fail percentage
	// the default is 75/100
	maxFailPercentage int
	// windowInterval decides when to reset counter if the state is StateClosed
	// the default is 5minutes
	windowInterval time.Duration
	// timeout decides when to turn StateOpen to StateHalfOpen
	// the default is 60s
	timeout time.Duration
	// maxRequests decides when to turn StateHalfOpen to StateClosed
	maxRequests int
}

type circuitBreaker struct {
	// settings
	breakerSetting
	// read and write lock
	mu sync.Mutex
	// the breaker's state: closed, open, half-open
	state state
	// expiry time determines whether to enter the next generation
	// if in StateClosed, it will be now + windowInterval
	// if in StateOpen, it will be now + timeout
	// if in StateHalfOpen. it will be zero
	expiry time.Time
	// generation decide whether add the afterRequest's request to counter
	generation uint64
	// counter
	counter counter
}

func newRegionBreaker(set breakerSetting) (re *circuitBreaker) {
	re = new(circuitBreaker)
	re.breakerSetting = set
	return
}

func defaultRegionBreaker() *circuitBreaker {
	defaultSet := breakerSetting{
		backupEndpoint:    defaultBackupEndpoint,
		maxFailNum:        defaultMaxFailNum,
		maxFailPercentage: defaultMaxFailPercentage,
		windowInterval:    defaultWindowLength,
		timeout:           defaultTimeout,
	}
	return newRegionBreaker(defaultSet)
}

// currentState return the current state.
//  if in StateClosed and now is over expiry time, it will turn to a new generation.
//  if in StateOpen and now is over expiry time, it will turn to StateHalfOpen
func (s *circuitBreaker) currentState(now time.Time) (state, uint64) {
	switch s.state {
	case StateClosed:
		if s.expiry.Before(now) {
			s.toNewGeneration(now)
		}
	case StateOpen:
		if s.expiry.Before(now) {
			s.setState(StateHalfOpen, now)
		}
	}
	return s.state, s.generation
}

// setState set the circuitBreaker's state to newState
// and turn to new generation
func (s *circuitBreaker) setState(newState state, now time.Time) {
	if s.state == newState {
		return
	}
	s.state = newState
	s.toNewGeneration(now)
}

// toNewGeneration will increase the generation and clear the counter.
// it also will reset the expiry
func (s *circuitBreaker) toNewGeneration(now time.Time) {
	s.generation++
	s.counter.clear()
	var zero time.Time
	switch s.state {
	case StateClosed:
		s.expiry = now.Add(s.windowInterval)
	case StateOpen:
		s.expiry = now.Add(s.timeout)
	default: // StateHalfOpen
		s.expiry = zero
	}
}

// beforeRequest return the current generation; if the breaker is in StateOpen, it will also return an errOpenState
func (s *circuitBreaker) beforeRequest() (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	state, generation := s.currentState(now)
	//log.Println(s.counter)
	if state == StateOpen {
		return generation, errOpenState
	}
	return generation, nil
}

func (s *circuitBreaker) afterRequest(before uint64, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	state, generation := s.currentState(now)
	// the breaker has entered the next generation, the current results abandon.
	if generation != before {
		return
	}
	if success {
		s.onSuccess(state, now)
	} else {
		s.onFailure(state, now)
	}
}

func (s *circuitBreaker) onSuccess(state state, now time.Time) {
	switch state {
	case StateClosed:
		s.counter.onSuccess()
	case StateHalfOpen:
		s.counter.onSuccess()
		// The conditions for closing breaker are met
		if s.counter.all-s.counter.failures >= s.maxRequests {
			s.setState(StateClosed, now)
		}
	}
}

func (s *circuitBreaker) readyToOpen(c counter) bool {
	failPre := float64(c.failures) / float64(c.all)
	return (c.failures >= s.maxFailNum && failPre >= float64(s.maxFailPercentage)/100.0) ||
		c.consecutiveFailures > 5
}

func (s *circuitBreaker) onFailure(state state, now time.Time) {
	switch state {
	case StateClosed:
		s.counter.onFailure()
		if f := s.readyToOpen(s.counter); f {
			s.setState(StateOpen, now)
		}
	case StateHalfOpen:
		s.setState(StateOpen, now)
	}
}

// checkEndpoint
// valid: cvm.ap-shanghai.tencentcloudapi.com, cvm.ap-shenzhen-fs.tencentcloudapi.com，cvm.tencentcloudapi.com
// invalid: cvm.tencentcloud.com
func checkEndpoint(endpoint string) bool {
	ss := strings.Split(endpoint, ".")
	if len(ss) != 4 && len(ss) != 3 {
		return false
	}
	if ss[len(ss)-2] != "tencentcloudapi" {
		return false
	}
	// ap-beijing
	if len(ss) == 4 && len(strings.Split(ss[1], "-")) < 2 {
		return false
	}
	return true
}

func renewUrl(oldDomain, region string) string {
	ss := strings.Split(oldDomain, ".")
	if len(ss) == 3 {
		ss = append([]string{ss[0], region}, ss[1:]...)
	} else if len(ss) == 4 {
		ss[1] = region
	}
	newDomain := strings.Join(ss, ".")
	return newDomain
}
