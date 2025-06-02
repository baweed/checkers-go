package network

import (
	"log"
	"net/http"
	"sync"

	"Shashki/game/core"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type GameSession struct {
	Board   *core.Board
	Clients map[*websocket.Conn]string
	mu      sync.Mutex
}

func NewGameSession() *GameSession {
	return &GameSession{
		Board:   core.NewBoard(),
		Clients: make(map[*websocket.Conn]string),
	}
}

func HandleWebSocket(c *gin.Context, session *GameSession) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer func() {
		session.mu.Lock()
		delete(session.Clients, conn)
		session.mu.Unlock()
		conn.Close()
	}()

	// Регистрация игрока
	session.mu.Lock()
	var playerColor string
	switch len(session.Clients) {
	case 0:
		playerColor = "black"
	case 1:
		playerColor = "white"
	default:
		conn.WriteJSON(Message{Type: "error", Content: "Game is full"})
		session.mu.Unlock()
		return
	}
	session.Clients[conn] = playerColor
	session.mu.Unlock()

	// Отправка начального состояния
	conn.WriteJSON(createGameState(session, playerColor))

	// Обработка сообщений
	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}

		switch msg.Type {
		case "move":
			if move, ok := msg.Content.(map[string]interface{}); ok {
				from := move["from"].(map[string]interface{})
				to := move["to"].(map[string]interface{})

				coreMove := core.Move{
					From: core.Position{
						X: int(from["x"].(float64)),
						Y: int(from["y"].(float64)),
					},
					To: core.Position{
						X: int(to["x"].(float64)),
						Y: int(to["y"].(float64)),
					},
				}

				session.processMove(conn, coreMove)
			}
		}
	}
}

func (s *GameSession) processMove(conn *websocket.Conn, move core.Move) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Проверка текущего игрока
	if s.Clients[conn] != s.Board.CurrentPlayer() {
		conn.WriteJSON(Message{
			Type:    "error",
			Content: "Not your turn",
		})
		return
	}

	// Применение хода
	if s.Board.ApplyMove(move) {
		// Рассылка нового состояния
		for client, color := range s.Clients {
			state := createGameState(s, color)
			if err := client.WriteJSON(state); err != nil {
				log.Printf("Error sending state: %v", err)
				delete(s.Clients, client)
				client.Close()
			}
		}
	} else {
		conn.WriteJSON(Message{
			Type:    "error",
			Content: "Invalid move",
		})
	}
}

func createGameState(session *GameSession, playerColor string) Message {
	return Message{
		Type: "state",
		Content: GameState{
			Board:         session.Board.Grid,
			CurrentPlayer: session.Board.CurrentPlayer(),
			YourColor:     playerColor,
			Winner:        getWinner(session.Board),
		},
	}
}

func getWinner(board *core.Board) string {
	if board.IsGameOver() {
		if board.CurrentPlayer() == "black" {
			return "white"
		}
		return "black"
	}
	return ""
}
