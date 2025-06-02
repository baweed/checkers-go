document.addEventListener("DOMContentLoaded", () => {
    // Получаем элементы DOM
    const boardElement = document.getElementById("board");
    const roomIdInput = document.getElementById("roomId");
    const joinRoomBtn = document.getElementById("joinRoom");
    const statusElement = document.getElementById("status") || createStatusElement();
    const playersDisplay = document.getElementById("players-display") || createPlayersDisplay();
    const gameInfoElement = document.getElementById("game-info") || createGameInfoElement();

    // Состояние игры
    let socket = null;
    let boardState = Array(8).fill().map(() => Array(8).fill(0));
    let currentPlayer = 1;
    let myPlayerNumber = 0;
    let selectedPiece = null;
    let currentRoomId = null;
    let playersCount = 0;
    let gameReady = false;

    // Создаем недостающие элементы
    function createStatusElement() {
        const el = document.createElement("div");
        el.id = "status";
        document.body.prepend(el);
        return el;
    }

    function createPlayersDisplay() {
        const el = document.createElement("div");
        el.id = "players-display";
        document.body.append(el);
        return el;
    }

    function createGameInfoElement() {
        const el = document.createElement("div");
        el.id = "game-info";
        document.body.append(el);
        return el;
    }

    // Инициализация игры
    function initGame() {
        renderBoard();
        updateGameInfo();
        updatePlayersDisplay();
        updateStatus("Disconnected", "disconnected");
    }

    // Подключение к комнате
    joinRoomBtn.addEventListener("click", () => {
        const roomId = roomIdInput.value.trim() || "default";
        connectToRoom(roomId);
    });

    function connectToRoom(roomId) {
        if (socket) {
            socket.close();
        }

        socket = new WebSocket(`ws://${window.location.host}/ws?room=${roomId}`);
        currentRoomId = roomId;

        socket.onopen = () => {
            console.log("WebSocket connected");
            updateStatus("Connected to room: " + roomId, "connected");
        };

        socket.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                console.log("Received:", data);
                handleServerMessage(data);
            } catch (err) {
                console.error("Error parsing message:", err);
            }
        };

        socket.onclose = () => {
            updateStatus("Disconnected", "disconnected");
            resetGameState();
        };

        socket.onerror = (error) => {
            console.error("WebSocket error:", error);
            updateStatus("Connection error", "error");
        };
    }

    // Обработчик сообщений от сервера
    function handleServerMessage(data) {
        switch (data.type) {
            case "connection_ack":
                handleConnectionAck(data);
                break;
            case "game_start":
                handleGameStart(data);
                break;
            case "players_update":
                handlePlayersUpdate(data);
                break;
            case "update":
                handleGameUpdate(data);
                break;
            case "error":
                showError(data.message);
                break;
            default:
                console.warn("Unknown message type:", data.type);
        }
    }

    function handleConnectionAck(data) {
        myPlayerNumber = data.yourPlayerNumber;
        playersCount = data.totalPlayers;
        gameReady = data.gameReady;
        
        if (data.board) {
            boardState = data.board;
        }
        currentPlayer = data.currentPlayer;

        updatePlayersDisplay();
        updateGameInfo();
        renderBoard();
    }

    function handleGameStart(data) {
        gameReady = true;
        playersCount = 2;
        updateGameInfo(data.yourTurn ? "Your turn!" : "Opponent's turn");
        updatePlayersDisplay();
        showMessage("Game started!");
    }

    function handlePlayersUpdate(data) {
        playersCount = data.count;
        gameReady = data.gameReady;
        updatePlayersDisplay();
        
        if (!gameReady) {
            updateGameInfo(`Waiting for opponent (${playersCount}/2)`);
        }
    }

    function handleGameUpdate(data) {
        boardState = data.board;
        currentPlayer = data.currentPlayer;
        
        if (data.captures) {
            animateCaptures(data.captures);
        }
        renderBoard();
        updateGameInfo();
    }

    // Обработчик кликов по доске
    function handleCellClick(x, y) {
        if (!isMoveAllowed()) return;

        const piece = boardState[y][x];
        const cell = document.querySelector(`[data-x="${x}"][data-y="${y}"]`);

        if (selectedPiece) {
            processMoveAttempt(x, y, piece, cell);
        } else {
            processPieceSelection(x, y, piece, cell);
        }
    }

    function isMoveAllowed() {
        if (!socket || socket.readyState !== WebSocket.OPEN) {
            showError("No connection to server");
            return false;
        }

        if (!gameReady) {
            showError("Game not ready");
            return false;
        }

        if (currentPlayer !== myPlayerNumber) {
            showError("Not your turn!");
            return false;
        }

        return true;
    }

    function processMoveAttempt(x, y, piece, cell) {
        if (x === selectedPiece.x && y === selectedPiece.y) {
            cell.classList.remove("selected");
            selectedPiece = null;
            return;
        }

        if (piece !== 0) {
            showError("Select empty cell to move");
            return;
        }

        sendMoveToServer(x, y);
    }

    function processPieceSelection(x, y, piece, cell) {
        if (piece === 0) return;

        const isMyPiece = (myPlayerNumber === 1 && (piece === 1 || piece === 3)) || 
                         (myPlayerNumber === 2 && (piece === 2 || piece === 4));
        
        if (isMyPiece) {
            document.querySelector(".selected")?.classList.remove("selected");
            cell.classList.add("selected");
            selectedPiece = { x, y };
        }
    }

    function sendMoveToServer(x, y) {
        const moveData = {
            type: "move",
            from: { x: selectedPiece.x, y: selectedPiece.y },
            to: { x, y }
        };

        console.log("Sending move:", moveData);
        
        try {
            socket.send(JSON.stringify(moveData));
            boardElement.classList.add("waiting");
        } catch (err) {
            console.error("Error sending move:", err);
            showError("Connection error");
        } finally {
            document.querySelector(".selected")?.classList.remove("selected");
            selectedPiece = null;
        }
    }

    // Визуальные функции
    function renderBoard() {
        boardElement.innerHTML = '';
        
        for (let y = 0; y < 8; y++) {
            for (let x = 0; x < 8; x++) {
                const cell = document.createElement("div");
                cell.className = `cell ${(x + y) % 2 === 0 ? "light" : "dark"}`;
                cell.dataset.x = x;
                cell.dataset.y = y;

                const piece = boardState[y][x];
                if (piece !== 0) {
                    const pieceElement = document.createElement("div");
                    pieceElement.className = "piece";
                    
                    if (piece === 1) pieceElement.classList.add("black");
                    else if (piece === 2) pieceElement.classList.add("white");
                    else if (piece === 3) pieceElement.classList.add("black", "king");
                    else if (piece === 4) pieceElement.classList.add("white", "king");

                    cell.appendChild(pieceElement);
                }

                cell.addEventListener("click", () => handleCellClick(x, y));
                boardElement.appendChild(cell);
            }
        }
        boardElement.classList.remove("waiting");
    }

    function animateCaptures(captures) {
        captures.forEach(cap => {
            const [x, y] = cap;
            const cell = document.querySelector(`[data-x="${x}"][data-y="${y}"]`);
            if (cell) {
                cell.classList.add("captured");
                setTimeout(() => {
                    cell.classList.remove("captured");
                    cell.innerHTML = '';
                }, 300);
            }
        });
    }

    function updateGameInfo(message) {
        if (!message) {
            message = currentPlayer === myPlayerNumber ? "Your turn!" : "Opponent's turn";
        }
        gameInfoElement.textContent = message;
        gameInfoElement.style.color = currentPlayer === myPlayerNumber ? "green" : "red";
    }

    function updatePlayersDisplay() {
        playersDisplay.textContent = `Players: ${playersCount}/2`;
        playersDisplay.className = gameReady ? "players-ready" : "players-waiting";
        boardElement.style.pointerEvents = gameReady ? "auto" : "none";
    }

    function updateStatus(message, type) {
        statusElement.textContent = message;
        statusElement.className = type;
    }

    function showError(message) {
        const errorElement = document.createElement("div");
        errorElement.className = "error-message";
        errorElement.textContent = message;
        document.body.appendChild(errorElement);
        setTimeout(() => errorElement.remove(), 3000);
    }

    function showMessage(message) {
        const msgElement = document.createElement("div");
        msgElement.className = "game-message";
        msgElement.textContent = message;
        document.body.appendChild(msgElement);
        setTimeout(() => msgElement.remove(), 2000);
    }

    function resetGameState() {
        boardState = Array(8).fill().map(() => Array(8).fill(0));
        currentPlayer = 1;
        myPlayerNumber = 0;
        selectedPiece = null;
        playersCount = 0;
        gameReady = false;
        updatePlayersDisplay();
        updateGameInfo("Disconnected");
        renderBoard();
    }

    // Инициализация
    initGame();
});