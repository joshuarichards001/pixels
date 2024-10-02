package main

type InitialMessage struct {
	Type        string `json:"type"`
	Data        string `json:"data"`
	ClientCount int    `json:"clientCount"`
}

type OutgoingMessage struct {
	Type        string       `json:"type"`
	Data        UpdatedColor `json:"data"`
	ClientCount int          `json:"clientCount"`
}

type IncomingMessage struct {
	Type string       `json:"type"`
	Data UpdatedColor `json:"data"`
}

type UpdatedColor struct {
	Index int    `json:"index"`
	Color string `json:"color"`
}
