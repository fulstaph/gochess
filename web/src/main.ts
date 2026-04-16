import { Board } from "./board";
import { GameClock } from "./clock";
import { GameHistoryPanel } from "./history";
import { Lobby } from "./lobby";
import { SoundPlayer } from "./sounds";
import { Settings, applyTheme } from "./settings";
import { ChessSocket } from "./socket";
import { GameState, ServerMessage } from "./types";

// ---- DOM elements ----

const lobbyEl = document.getElementById("lobby")!;
const gameAreaEl = document.getElementById("game-area")!;
const boardEl = document.getElementById("board")!;
const thinkingEl = document.getElementById("thinking")!;
const turnIndicator = document.getElementById("turn-indicator")!;
const checkIndicator = document.getElementById("check-indicator")!;
const resultBanner = document.getElementById("result-banner")!;
const moveList = document.getElementById("move-list")!;
const flipBtn = document.getElementById("flip-btn")!;
const resignBtn = document.getElementById("resign-btn")!;
const toLobbyBtn = document.getElementById("to-lobby-btn")!;
const opponentBanner = document.getElementById("opponent-banner")!;
const drawOfferBanner = document.getElementById("draw-offer-banner")!;
const drawAcceptBtn = document.getElementById("draw-accept-btn")!;
const drawDeclineBtn = document.getElementById("draw-decline-btn")!;
const undoBtn = document.getElementById("undo-btn")!;
const drawOfferBtn = document.getElementById("draw-offer-btn")!;
const claimDrawBtn = document.getElementById("claim-draw-btn")!;
const ratingBanner = document.getElementById("rating-banner")!;
const whiteClockEl = document.getElementById("clock-white")!;
const blackClockEl = document.getElementById("clock-black")!;
const clockTopSlot = document.getElementById("clock-top-slot")!;
const clockBottomSlot = document.getElementById("clock-bottom-slot")!;
const playerNameTopEl = document.getElementById("player-name-top")!;
const playerNameBottomEl = document.getElementById("player-name-bottom")!;
const historyPanelEl = document.getElementById("history-panel")!;
const settingsBarEl = document.getElementById("settings-bar")!;
const soundToggleEl = document.getElementById("sound-toggle") as HTMLInputElement;
const themeSelectEl = document.getElementById("theme-select") as HTMLSelectElement;
const autoFlipToggleEl = document.getElementById("auto-flip-toggle") as HTMLInputElement;
const quickMatchBtn = document.getElementById("quick-match-btn") as HTMLButtonElement;
const quickMatchTcEl = document.getElementById("quick-match-tc") as HTMLSelectElement;
const matchWaitingEl = document.getElementById("match-waiting")!;

// ---- State ----

let gameState: GameState | null = null;
let myPlayerID = "";
let myDisplayName = "";
let isVsAI = false;

// ---- Settings ----

const sounds = new SoundPlayer();

const settings = new Settings((data) => {
  sounds.enabled = data.sound;
  applyTheme(data.theme);
  soundToggleEl.checked = data.sound;
  themeSelectEl.value = data.theme;
  autoFlipToggleEl.checked = data.autoFlip;
});

soundToggleEl.addEventListener("change", () => { settings.sound = soundToggleEl.checked; });
themeSelectEl.addEventListener("change", () => {
  settings.theme = themeSelectEl.value as import("./settings").BoardTheme;
});
autoFlipToggleEl.addEventListener("change", () => { settings.autoFlip = autoFlipToggleEl.checked; });

// ---- History ----

const historyPanel = new GameHistoryPanel(historyPanelEl);

// ---- Clocks ----

const whiteClock = new GameClock(whiteClockEl);
const blackClock = new GameClock(blackClockEl);

// ---- Board ----

const board = new Board(boardEl, (from, to, promotion) => {
  socket.send({ type: "move", from, to, promotion });
});

// ---- Lobby ----

const lobby = new Lobby(lobbyEl, {
  onCreateRoom: ({ aiMode, aiDepth, color, timeControl }) => {
    isVsAI = aiMode !== "none" && aiMode !== "";
    socket.send({ type: "create_room", aiMode, aiDepth, color, timeControl });
  },
  onJoinRoom: (roomId) => {
    socket.send({ type: "join_room", roomId });
  },
  onRefresh: () => {
    socket.send({ type: "list_rooms" });
  },
});

// ---- Socket ----

const socket = new ChessSocket(handleMessage);

function handleMessage(msg: ServerMessage): void {
  switch (msg.type) {
    case "session":
      myPlayerID = msg.playerId;
      myDisplayName = msg.displayName;
      playerNameBottomEl.textContent = msg.displayName;
      lobby.setDisplayName(msg.displayName);
      socket.send({ type: "list_rooms" });
      historyPanel.load(msg.playerId);
      break;

    case "auth_ok":
      myPlayerID = msg.playerId;
      myDisplayName = msg.displayName;
      playerNameBottomEl.textContent = msg.displayName;
      lobby.setDisplayName(msg.displayName);
      historyPanel.load(msg.playerId);
      break;

    case "room_list":
      lobby.renderRooms(msg.rooms);
      break;

    case "room_created":
    case "room_joined": {
      board.setFlipped(msg.playerColor === "black");
      setupPanels(msg.playerColor);
      playerNameBottomEl.textContent = myDisplayName;
      drawOfferBtn.classList.remove("hidden");
      if (isVsAI) undoBtn.classList.remove("hidden");
      else undoBtn.classList.add("hidden");
      if (msg.type === "room_joined") {
        playerNameTopEl.textContent = msg.opponentName;
        opponentBanner.textContent = `Opponent: ${msg.opponentName}`;
        opponentBanner.classList.remove("hidden");
      } else {
        playerNameTopEl.textContent = isVsAI ? "Computer" : "Waiting...";
      }
      showGame();
      break;
    }

    case "match_found": {
      isVsAI = false;
      board.setFlipped(msg.playerColor === "black");
      setupPanels(msg.playerColor);
      playerNameBottomEl.textContent = myDisplayName;
      playerNameTopEl.textContent = msg.opponentName;
      drawOfferBtn.classList.remove("hidden");
      undoBtn.classList.add("hidden");
      opponentBanner.textContent = `Opponent: ${msg.opponentName}`;
      opponentBanner.classList.remove("hidden");
      matchWaitingEl.classList.add("hidden");
      quickMatchBtn.disabled = false;
      showGame();
      break;
    }

    case "match_waiting":
      matchWaitingEl.textContent = `Searching for a ${msg.timeControl} game...`;
      matchWaitingEl.classList.remove("hidden");
      quickMatchBtn.disabled = true;
      break;

    case "match_cancelled":
      matchWaitingEl.classList.add("hidden");
      quickMatchBtn.disabled = false;
      break;

    case "state": {
      const prev = gameState;
      gameState = msg;
      thinkingEl.classList.add("hidden");
      board.clearSelection();

      playSoundForState(prev, msg);

      // Auto-flip when it's the player's turn and autoFlip is on.
      if (settings.autoFlip && msg.playerColor && msg.playerColor !== "spectator") {
        const isMyTurn =
          (msg.turn === "white" && msg.playerColor === "white") ||
          (msg.turn === "black" && msg.playerColor === "black");
        board.setFlipped(!isMyTurn ? msg.playerColor === "white" : msg.playerColor === "black");
      }

      board.render(gameState);
      updateSidebar();
      syncClocks(gameState);
      if (gameAreaEl.classList.contains("hidden")) showGame();
      break;
    }

    case "thinking":
      thinkingEl.classList.remove("hidden");
      break;

    case "draw_offered":
      drawOfferBanner.classList.remove("hidden");
      break;

    case "rating_update": {
      const sign = msg.delta >= 0 ? "+" : "";
      ratingBanner.textContent = `Rating: ${msg.newRating} (${sign}${msg.delta})`;
      ratingBanner.classList.remove("hidden");
      break;
    }

    case "opponent_disconnected":
      opponentBanner.textContent = "Opponent disconnected — waiting 60s...";
      opponentBanner.classList.remove("hidden");
      break;

    case "opponent_reconnected":
      opponentBanner.textContent = gameState?.playerColor ? `Opponent reconnected` : "";
      break;

    case "error":
      console.error("Server error:", msg.message);
      break;
  }
}

function playSoundForState(prev: GameState | null, next: GameState): void {
  if (next.moveHistory.length <= (prev?.moveHistory.length ?? 0)) return;

  if (next.isGameOver) {
    sounds.playGameEnd();
    return;
  }
  if (next.isCheck) {
    sounds.playCheck();
    return;
  }
  // Detect capture: last move landed on a non-empty square.
  if (prev && next.lastMove) {
    const [tr, tc] = squareToCoords(next.lastMove.to);
    const wasOccupied = prev.board[tr]?.[tc] !== "";
    if (wasOccupied) {
      sounds.playCapture();
      return;
    }
    // Castle detection: king moved two squares.
    const [fr, fc] = squareToCoords(next.lastMove.from);
    const [, tc2] = squareToCoords(next.lastMove.to);
    if ((prev.board[fr][fc] === "K" || prev.board[fr][fc] === "k") && Math.abs(fc - tc2) === 2) {
      sounds.playCastle();
      return;
    }
  }
  sounds.playMove();
}

function squareToCoords(sq: string): [number, number] {
  const c = sq.charCodeAt(0) - 97;
  const r = 8 - parseInt(sq[1], 10);
  return [r, c];
}

function setupPanels(playerColor: string): void {
  const myClockEl = playerColor === "black" ? blackClockEl : whiteClockEl;
  const opClockEl = playerColor === "black" ? whiteClockEl : blackClockEl;
  clockBottomSlot.appendChild(myClockEl);
  clockTopSlot.appendChild(opClockEl);
}

function syncClocks(state: GameState): void {
  gameAreaEl.classList.toggle("has-clock", state.hasClock ?? false);
  if (!state.hasClock) {
    whiteClock.stop();
    blackClock.stop();
    return;
  }
  const whiteTurn = state.turn === "white" && !state.isGameOver;
  const blackTurn = state.turn === "black" && !state.isGameOver;
  whiteClock.sync(state.whiteMs ?? 0, whiteTurn);
  blackClock.sync(state.blackMs ?? 0, blackTurn);
}

function updateSidebar(): void {
  if (!gameState) return;

  const turnText = gameState.turn === "white" ? "White to move" : "Black to move";
  turnIndicator.textContent = gameState.isGameOver ? "" : turnText;
  turnIndicator.className = gameState.isGameOver ? "" : gameState.turn;

  if (gameState.isCheck && !gameState.isGameOver) {
    checkIndicator.classList.remove("hidden");
  } else {
    checkIndicator.classList.add("hidden");
  }

  if (gameState.isGameOver && gameState.result) {
    resultBanner.textContent = gameState.result;
    resultBanner.classList.remove("hidden");
    drawOfferBtn.classList.add("hidden");
    claimDrawBtn.classList.add("hidden");
    drawOfferBanner.classList.add("hidden");
    whiteClock.stop();
    blackClock.stop();
    if (myPlayerID) {
      setTimeout(() => historyPanel.load(myPlayerID), 1500);
    }
  } else {
    resultBanner.classList.add("hidden");
  }

  if (gameState.claimableDraw) {
    claimDrawBtn.title = gameState.claimableDraw;
    claimDrawBtn.classList.remove("hidden");
  } else {
    claimDrawBtn.classList.add("hidden");
  }

  if (!gameState.drawOffered) {
    drawOfferBanner.classList.add("hidden");
  }

  gameAreaEl.classList.toggle("white-turn", gameState.turn === "white" && !gameState.isGameOver);
  gameAreaEl.classList.toggle("black-turn", gameState.turn === "black" && !gameState.isGameOver);

  moveList.innerHTML = "";
  for (const entry of gameState.moveHistory) {
    // Entry format: "1. e4 e5" or "5. Nf3"
    const parts = entry.split(" ");
    const numCell = document.createElement("span");
    numCell.className = "move-num";
    numCell.textContent = parts[0] ?? "";
    const whiteCell = document.createElement("span");
    whiteCell.className = "move-white";
    whiteCell.textContent = parts[1] ?? "";
    const blackCell = document.createElement("span");
    blackCell.className = "move-black";
    blackCell.textContent = parts[2] ?? "";
    moveList.appendChild(numCell);
    moveList.appendChild(whiteCell);
    moveList.appendChild(blackCell);
  }
  moveList.scrollTop = moveList.scrollHeight;
}

function showGame(): void {
  lobbyEl.classList.add("hidden");
  gameAreaEl.classList.remove("hidden");
  settingsBarEl.classList.remove("hidden");
}

function showLobby(): void {
  gameAreaEl.classList.add("hidden");
  gameAreaEl.classList.remove("has-clock", "white-turn", "black-turn");
  gameState = null;
  isVsAI = false;
  opponentBanner.classList.add("hidden");
  drawOfferBanner.classList.add("hidden");
  ratingBanner.classList.add("hidden");
  drawOfferBtn.classList.add("hidden");
  undoBtn.classList.add("hidden");
  claimDrawBtn.classList.add("hidden");
  playerNameTopEl.textContent = "";
  whiteClock.stop();
  blackClock.stop();
  lobbyEl.classList.remove("hidden");
  socket.send({ type: "list_rooms" });
}

// ---- Controls ----

flipBtn.addEventListener("click", () => board.flip());

resignBtn.addEventListener("click", () => {
  if (gameState && !gameState.isGameOver) {
    socket.send({ type: "resign" });
  }
});

undoBtn.addEventListener("click", () => {
  if (gameState && !gameState.isGameOver && isVsAI) {
    socket.send({ type: "undo" });
  }
});

toLobbyBtn.addEventListener("click", () => showLobby());

drawOfferBtn.addEventListener("click", () => {
  socket.send({ type: "draw_offer" });
  (drawOfferBtn as HTMLButtonElement).disabled = true;
  setTimeout(() => { (drawOfferBtn as HTMLButtonElement).disabled = false; }, 5000);
});

drawAcceptBtn.addEventListener("click", () => {
  socket.send({ type: "draw_response", accept: true });
  drawOfferBanner.classList.add("hidden");
});

drawDeclineBtn.addEventListener("click", () => {
  socket.send({ type: "draw_response", accept: false });
  drawOfferBanner.classList.add("hidden");
});

claimDrawBtn.addEventListener("click", () => {
  socket.send({ type: "draw_offer" });
});

// ---- TC Card Grid ----

document.getElementById("tc-grid")?.addEventListener("click", (e) => {
  const card = (e.target as HTMLElement).closest<HTMLElement>(".tc-card");
  if (!card?.dataset.tc) return;
  quickMatchTcEl.value = card.dataset.tc;
  quickMatchBtn.click();
});

// ---- Quick Match ----

quickMatchBtn.addEventListener("click", () => {
  if (quickMatchBtn.disabled) {
    // Cancel the search.
    socket.send({ type: "cancel_match" });
    return;
  }
  socket.send({ type: "find_game", timeControl: quickMatchTcEl.value });
});

// ---- Keyboard ----

document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") {
    document.getElementById("promotion-dialog")?.classList.add("hidden");
  }
});
