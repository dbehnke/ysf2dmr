package network

import "time"

// Timer provides millisecond precision timing equivalent to C++ CTimer
type Timer struct {
	ticksPerSec  int
	timeoutTicks int
	currentTicks int
	running      bool
	startTime    time.Time
}

// NewTimer creates a new timer with specified resolution
// Equivalent to C++ CTimer(ticksPerSec, secs, msecs)
func NewTimer(ticksPerSec int, secs, msecs int) *Timer {
	timer := &Timer{
		ticksPerSec: ticksPerSec,
	}

	if secs > 0 || msecs > 0 {
		timer.SetTimeout(secs, msecs)
	}

	return timer
}

// SetTimeout sets the timeout duration
// Equivalent to C++ CTimer::setTimeout()
func (t *Timer) SetTimeout(secs, msecs int) {
	t.timeoutTicks = (secs * t.ticksPerSec) + (msecs * t.ticksPerSec / 1000)
}

// IsRunning returns true if timer is currently running
// Equivalent to C++ CTimer::isRunning()
func (t *Timer) IsRunning() bool {
	return t.running
}

// Start starts the timer with optional timeout
// Equivalent to C++ CTimer::start()
func (t *Timer) Start(secs, msecs int) {
	if secs > 0 || msecs > 0 {
		t.SetTimeout(secs, msecs)
	}
	t.currentTicks = 0
	t.running = true
	t.startTime = time.Now()
}

// Stop stops the timer
// Equivalent to C++ CTimer::stop()
func (t *Timer) Stop() {
	t.running = false
}

// HasExpired checks if timer has expired
// Equivalent to C++ CTimer::hasExpired()
func (t *Timer) HasExpired() bool {
	// Timer with zero timeout should never be considered expired unless explicitly started
	if t.timeoutTicks == 0 {
		return false
	}

	if !t.running && t.currentTicks < t.timeoutTicks {
		return false // Not running and hasn't reached timeout
	}
	return t.currentTicks >= t.timeoutTicks
}

// Clock advances the timer by specified ticks (typically milliseconds)
// Equivalent to C++ CTimer::clock()
func (t *Timer) Clock(ticks int) {
	if !t.running {
		return
	}

	t.currentTicks += ticks

	// Auto-stop if expired
	if t.currentTicks >= t.timeoutTicks {
		t.running = false
	}
}

// ClockAuto automatically calculates elapsed time since start
func (t *Timer) ClockAuto() {
	if !t.running {
		return
	}

	elapsed := time.Since(t.startTime)
	elapsedTicks := int(elapsed.Nanoseconds()) * t.ticksPerSec / 1000000000

	if elapsedTicks >= t.timeoutTicks {
		t.running = false
		t.currentTicks = t.timeoutTicks
	} else {
		t.currentTicks = elapsedTicks
	}
}

// GetElapsedMS returns elapsed time in milliseconds
func (t *Timer) GetElapsedMS() int {
	if !t.running {
		return 0
	}
	return t.currentTicks * 1000 / t.ticksPerSec
}

// GetRemainingMS returns remaining time in milliseconds
func (t *Timer) GetRemainingMS() int {
	if !t.running {
		return 0
	}
	remaining := t.timeoutTicks - t.currentTicks
	if remaining <= 0 {
		return 0
	}
	return remaining * 1000 / t.ticksPerSec
}