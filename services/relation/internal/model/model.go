package model

const (
	HandlePending int32 = 0
	HandleAgree   int32 = 1
	HandleRefuse  int32 = -1

	// AddSourceApply is the friend row created when a request is accepted.
	AddSourceApply int32 = 2
)
