package execution

import "errors"

var (
	// ErrNilConfig is returned when the configuration is nil.
	ErrNilConfig = errors.New("execution mode config is nil")

	// ErrNilIterationFunc is returned when the iteration function is nil.
	ErrNilIterationFunc = errors.New("iteration function is nil")

	// ErrNoStages is returned when no stages are defined for ramping modes.
	ErrNoStages = errors.New("no stages defined for ramping mode")

	// ErrInvalidRate is returned when the rate is invalid.
	ErrInvalidRate = errors.New("invalid rate: must be positive")

	// ErrInvalidTimeUnit is returned when the time unit is invalid.
	ErrInvalidTimeUnit = errors.New("invalid time unit: must be positive")

	// ErrModeNotRunning is returned when trying to control a mode that is not running.
	ErrModeNotRunning = errors.New("execution mode is not running")

	// ErrModeAlreadyRunning is returned when trying to start a mode that is already running.
	ErrModeAlreadyRunning = errors.New("execution mode is already running")

	// ErrScaleNotSupported is returned when scaling is not supported by the mode.
	ErrScaleNotSupported = errors.New("scaling is not supported by this execution mode")

	// ErrPauseNotSupported is returned when pausing is not supported by the mode.
	ErrPauseNotSupported = errors.New("pausing is not supported by this execution mode")
)
