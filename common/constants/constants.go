package constants

// Named pipe constants.
const (
	// DefaultPipeName is the default named pipe address.
	DefaultPipeName = `\\.\pipe\ees`

	// PipeOpenTimeout is the maximum time (in seconds) to wait for a pipe connection.
	PipeOpenTimeout = 5

	// PipeBufferSize is the read/write buffer size for named pipe communication.
	PipeBufferSize = 4096
)

// Result values returned by the Agent.
const (
	ResultAllow = "Allow"
	ResultDeny  = "Deny"
	ResultError = "Error"
)

// Log levels (common interface keys).
const (
	LogLevelInfo  = "INFO"
	LogLevelWarn  = "WARN"
	LogLevelError = "ERROR"
)

// Agent exit / error codes.
const (
	ExitCodeOK    = 0
	ExitCodeError = 1
)
