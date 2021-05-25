package manager

// Response - y/n kinda situation
type Response string

const (
	// YES -
	YES Response = "Y"

	// NO -
	NO Response = "N"
)

// Status -
type Status string

const (
	// NEW -
	NEW Status = "NEW"

	// QUEUE -
	QUEUE Status = "QUEUE"

	// SENT -
	SENT Status = "SENT"

	// UPDATED -
	UPDATED Status = "UPDATED"

	// DEAD -
	DEAD Status = "DEAD"
)
