package core

type PieceType int

const (
	Empty PieceType = iota
	Black
	White
	BlackKing
	WhiteKing
)

type Position struct {
	X, Y int
}

type Move struct {
	From     Position
	To       Position
	Captures []Position
}

type Board struct {
	Grid          [8][8]PieceType
	currentPlayer string
	moveHistory   []Move
}

func NewBoard() *Board {
	b := &Board{
		currentPlayer: "black",
	}
	b.initializePieces()
	return b
}

func (b *Board) initializePieces() {
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if (x+y)%2 != 0 {
				if y < 3 {
					b.Grid[y][x] = Black
				} else if y > 4 {
					b.Grid[y][x] = White
				}
			}
		}
	}
}

func (b *Board) CurrentPlayer() string {
	return b.currentPlayer
}

func (b *Board) IsValidMove(move Move) bool {
	// Упрощенная проверка правил
	from := move.From
	to := move.To

	if from.X < 0 || from.X > 7 || from.Y < 0 || from.Y > 7 ||
		to.X < 0 || to.X > 7 || to.Y < 0 || to.Y > 7 {
		return false
	}

	if b.Grid[to.Y][to.X] != Empty {
		return false
	}

	piece := b.Grid[from.Y][from.X]
	if (piece == Black && b.currentPlayer != "black") ||
		(piece == White && b.currentPlayer != "white") {
		return false
	}

	return true
}

func (b *Board) ApplyMove(move Move) bool {
	if !b.IsValidMove(move) {
		return false
	}

	from := move.From
	to := move.To

	// Перемещаем фигуру
	b.Grid[to.Y][to.X] = b.Grid[from.Y][from.X]
	b.Grid[from.Y][from.X] = Empty

	// Превращение в дамки
	if b.Grid[to.Y][to.X] == Black && to.Y == 7 {
		b.Grid[to.Y][to.X] = BlackKing
	} else if b.Grid[to.Y][to.X] == White && to.Y == 0 {
		b.Grid[to.Y][to.X] = WhiteKing
	}

	// Меняем игрока
	if b.currentPlayer == "black" {
		b.currentPlayer = "white"
	} else {
		b.currentPlayer = "black"
	}

	b.moveHistory = append(b.moveHistory, move)
	return true
}

func (b *Board) IsGameOver() bool {
	// Упрощенная проверка окончания игры
	blackExists := false
	whiteExists := false

	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if b.Grid[y][x] == Black || b.Grid[y][x] == BlackKing {
				blackExists = true
			} else if b.Grid[y][x] == White || b.Grid[y][x] == WhiteKing {
				whiteExists = true
			}
		}
	}

	return !blackExists || !whiteExists
}
