package entity

// ErrorResponse is the standard API error payload.
type ErrorResponse struct {
	Msg string `json:"msg" example:"something went wrong"`
}

// HealthResponse is returned by the health check endpoint.
type HealthResponse struct {
	Msg string `json:"msg" example:"OK"`
}
