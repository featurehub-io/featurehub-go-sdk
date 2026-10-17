package models

// EdgeType identifies which edge connection strategy the SDK uses.
type EdgeType string

const (
	EdgeStreaming   EdgeType = "streaming"
	EdgeActiveRest  EdgeType = "active-rest"
	EdgePassiveRest EdgeType = "passive-rest"
)
