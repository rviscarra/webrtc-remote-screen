package api

type newSessionRequest struct {
	Offer  string `json:"offer"`
	Screen int    `json:"screen"`
}

type newSessionResponse struct {
	Answer string `json:"answer"`
}

type screenPayload struct {
	Index int `json:"index"`
}

type screensResponse struct {
	Screens []screenPayload `json:"screens"`
}
