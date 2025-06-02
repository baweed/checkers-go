package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type GameState struct {
	Board         [8][8]int `json:"board"`
	CurrentPlayer int       `json:"currentPlayer"` // 1 - черные, 2 - белые
}

type GameRoom struct {
	ID      string
	Clients map[*websocket.Conn]int // conn → playerNumber (1 или 2)
	Game    GameState
	mu      sync.Mutex
}

type MoveMessage struct {
	Type string `json:"type"`
	From struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"from"`
	To struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"to"`
}

var (
	rooms     = make(map[string]*GameRoom)
	roomMutex sync.Mutex
)

func createRoom(roomID string) *GameRoom {
	return &GameRoom{
		ID:      roomID,
		Clients: make(map[*websocket.Conn]int),
		Game: GameState{
			Board: [8][8]int{
				{0, 1, 0, 1, 0, 1, 0, 1},
				{1, 0, 1, 0, 1, 0, 1, 0},
				{0, 1, 0, 1, 0, 1, 0, 1},
				{0, 0, 0, 0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0, 0, 0, 0},
				{2, 0, 2, 0, 2, 0, 2, 0},
				{0, 2, 0, 2, 0, 2, 0, 2},
				{2, 0, 2, 0, 2, 0, 2, 0},
			},
			CurrentPlayer: 1,
		},
	}
}

func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	roomID := c.Query("room")
	if roomID == "" {
		roomID = "default"
	}

	roomMutex.Lock()
	room, exists := rooms[roomID]
	if !exists {
		room = createRoom(roomID)
		rooms[roomID] = room
		log.Printf("Created new room: %s", roomID)
	}
	roomMutex.Unlock()

	// Регистрация игрока с блокировкой
	room.mu.Lock()
	playerNumber := len(room.Clients) + 1
	if playerNumber > 2 {
		sendError(conn, "Room is full")
		room.mu.Unlock()
		return
	}

	// Сохраняем подключение
	room.Clients[conn] = playerNumber
	currentPlayers := len(room.Clients)
	isGameReady := currentPlayers == 2

	// Отправляем подтверждение подключения
	conn.WriteJSON(gin.H{
		"type":             "connection_ack",
		"yourPlayerNumber": playerNumber,
		"totalPlayers":     currentPlayers,
		"gameReady":        isGameReady,
		"currentPlayer":    room.Game.CurrentPlayer,
		"board":            room.Game.Board,
	})

	// Если подключился второй игрок - уведомляем всех
	if isGameReady {
		for client := range room.Clients {
			client.WriteJSON(gin.H{
				"type":     "game_start",
				"message":  "Game is ready!",
				"yourTurn": room.Clients[client] == room.Game.CurrentPlayer,
			})
		}
	}
	room.mu.Unlock()

	// Обработка сообщений
	for {
		var msg struct {
			Type string `json:"type"`
			// ... другие поля
		}

		if err := conn.ReadJSON(&msg); err != nil {
			room.removeClient(conn)
			break
		}

		switch msg.Type {
		case "move":
			var moveMsg MoveMessage
			if err := conn.ReadJSON(&moveMsg); err != nil {
				log.Println("Error reading move:", err)
				continue
			}
			room.handleMove(conn, moveMsg)
		case "get_players":
			room.sendPlayersUpdate(conn)
		}
	}
}

func (r *GameRoom) sendPlayersUpdate(conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	conn.WriteJSON(gin.H{
		"type":      "players_update",
		"count":     len(r.Clients),
		"gameReady": len(r.Clients) == 2,
	})
}

func (r *GameRoom) broadcastPlayersUpdate() {
	r.mu.Lock()
	defer r.mu.Unlock()

	playersCount := len(r.Clients)
	for client := range r.Clients {
		client.WriteJSON(gin.H{
			"type":      "players_update",
			"count":     playersCount,
			"gameReady": playersCount == 2,
		})
	}
}

func (r *GameRoom) removeClient(conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if playerNumber, exists := r.Clients[conn]; exists {
		delete(r.Clients, conn)
		playersCount := len(r.Clients)
		log.Printf("Player %d disconnected from room %s, remaining: %d", playerNumber, r.ID, playersCount)

		if playersCount == 0 {
			roomMutex.Lock()
			delete(rooms, r.ID)
			roomMutex.Unlock()
			return
		}

		// Если отключился текущий игрок - передаем ход
		if playerNumber == r.Game.CurrentPlayer {
			r.Game.CurrentPlayer = 3 - r.Game.CurrentPlayer
		}

		r.broadcastPlayersUpdate()
	}
}

func (r *GameRoom) handleMove(conn *websocket.Conn, msg MoveMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()

	playerNumber := r.Clients[conn]
	fromX, fromY := msg.From.X, msg.From.Y
	toX, toY := msg.To.X, msg.To.Y
	piece := r.Game.Board[fromY][fromX]

	// Проверка очереди хода
	if playerNumber != r.Game.CurrentPlayer {
		sendError(conn, "Not your turn")
		return
	}

	// Проверка принадлежности фишки
	if (playerNumber == 1 && (piece != 1 && piece != 3)) ||
		(playerNumber == 2 && (piece != 2 && piece != 4)) {
		sendError(conn, "You can only move your pieces")
		return
	}

	// Проверка валидности хода
	valid, captures := r.validateMove(piece, fromX, fromY, toX, toY)
	if !valid {
		sendError(conn, "Invalid move")
		return
	}

	// Выполнение хода
	r.executeMove(fromX, fromY, toX, toY, captures)

	// Отправка обновления
	r.broadcast(gin.H{
		"type":          "update",
		"board":         r.Game.Board,
		"currentPlayer": r.Game.CurrentPlayer,
		"captures":      captures,
		"message":       fmt.Sprintf("Player %d moved", playerNumber),
	})
}

func (r *GameRoom) executeMove(fromX, fromY, toX, toY int, captures [][2]int) {
	piece := r.Game.Board[fromY][fromX]

	// Удаление взятых фишек
	for _, cap := range captures {
		r.Game.Board[cap[1]][cap[0]] = 0
	}

	// Перемещение фишки
	r.Game.Board[toY][toX] = piece
	r.Game.Board[fromY][fromX] = 0

	// Превращение в дамку
	if (piece == 1 && toY == 7) || (piece == 2 && toY == 0) {
		r.Game.Board[toY][toX] += 2
	}

	// Смена игрока
	r.Game.CurrentPlayer = 3 - r.Game.CurrentPlayer
}

func (r *GameRoom) validateMove(piece, fromX, fromY, toX, toY int) (bool, [][2]int) {
	dx := toX - fromX
	dy := toY - fromY
	absDx := abs(dx)
	absDy := abs(dy)
	captures := [][2]int{}

	// Базовые проверки
	if fromX < 0 || fromX > 7 || fromY < 0 || fromY > 7 ||
		toX < 0 || toX > 7 || toY < 0 || toY > 7 ||
		r.Game.Board[toY][toX] != 0 ||
		absDx != absDy {
		return false, captures
	}

	// Проверка направления для обычных фишек
	if piece == 1 && dy < 1 { // Черные ходят только вниз
		return false, captures
	}
	if piece == 2 && dy > -1 { // Белые ходят только вверх
		return false, captures
	}

	// Проверка для обычных фишек
	if piece == 1 || piece == 2 {
		if absDx > 2 {
			return false, captures
		}

		if absDx == 2 {
			midX, midY := fromX+dx/2, fromY+dy/2
			midPiece := r.Game.Board[midY][midX]

			if (piece == 1 && (midPiece == 2 || midPiece == 4)) ||
				(piece == 2 && (midPiece == 1 || midPiece == 3)) {
				captures = append(captures, [2]int{midX, midY})
				return true, captures
			}
			return false, captures
		}
		return true, captures
	}

	// Проверка для дамок
	stepX := dx / absDx
	stepY := dy / absDy
	hasCapture := false

	for x, y := fromX+stepX, fromY+stepY; x != toX || y != toY; x, y = x+stepX, y+stepY {
		cell := r.Game.Board[y][x]
		if cell != 0 {
			if (piece == 3 && (cell == 2 || cell == 4)) ||
				(piece == 4 && (cell == 1 || cell == 3)) {
				if hasCapture {
					return false, captures
				}
				captures = append(captures, [2]int{x, y})
				hasCapture = true
			} else {
				return false, captures
			}
		}
	}

	return true, captures
}

func (r *GameRoom) broadcast(message interface{}) {
	for client := range r.Clients {
		if err := client.WriteJSON(message); err != nil {
			log.Println("Broadcast error:", err)
			delete(r.Clients, client)
			client.Close()
		}
	}
}

func sendError(conn *websocket.Conn, message string) {
	if err := conn.WriteJSON(gin.H{
		"type":    "error",
		"message": message,
	}); err != nil {
		log.Println("Error sending error message:", err)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
