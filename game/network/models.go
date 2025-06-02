package network

import "Shashki/game/core"

type Message struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
}

type GameState struct {
	Board         [8][8]core.PieceType `json:"board"`
	CurrentPlayer string               `json:"currentPlayer"`
	YourColor     string               `json:"yourColor"`
	Winner        string               `json:"winner,omitempty"`
}
