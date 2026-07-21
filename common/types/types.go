package types

// Request is sent by the Explorer Client to the Agent via Named Pipe.
type Request struct {
	// Path is the full path of the executable to verify and elevate.
	Path string `json:"Path"`
}

// Response is sent by the Agent back to the Explorer Client.
type Response struct {
	// Result is one of "Allow", "Deny", or "Error".
	Result string `json:"Result"`

	// Message provides a human-readable description of the result.
	Message string `json:"Message"`
}
