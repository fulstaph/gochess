package server

const ProtocolVersion = "1"

// ---- Client → Server messages ----

type ClientMessage struct {
	Type        string `json:"type"`
	V           string `json:"v,omitempty"`
	// move
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	Promotion   string `json:"promotion,omitempty"`
	// new_game (single-session legacy) / create_room
	AIMode      string `json:"aiMode,omitempty"`
	AIDepth     int    `json:"aiDepth,omitempty"`
	// create_room
	Color       string `json:"color,omitempty"`       // "white", "black", "random"
	TimeControl string `json:"timeControl,omitempty"` // preset name or "none"
	// join_room / spectate
	RoomID      string `json:"roomId,omitempty"`
	// draw_response
	Accept      bool   `json:"accept,omitempty"`
	// register / login
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	// find_game (matchmaking)
	// TimeControl reused
}

// ---- Server → Client messages ----

// StateMessage is sent after every move, new game, or resign.
type StateMessage struct {
	Type        string       `json:"type"`
	V           string       `json:"v"`
	Board       [8][8]string `json:"board"`
	Turn        string       `json:"turn"`
	MoveNumber  int          `json:"moveNumber"`
	IsCheck     bool         `json:"isCheck"`
	IsGameOver  bool         `json:"isGameOver"`
	Result      string       `json:"result"`
	LegalMoves  []LegalMove  `json:"legalMoves"`
	LastMove    *LegalMove   `json:"lastMove"`
	MoveHistory []string     `json:"moveHistory"`
	// Multiplayer fields (zero-value omitted for backwards compat)
	PlayerColor  string `json:"playerColor,omitempty"`  // "white" | "black" | "spectator"
	RoomID       string `json:"roomId,omitempty"`
	// Clock fields (zero when no time control)
	WhiteMs      int64  `json:"whiteMs,omitempty"`
	BlackMs      int64  `json:"blackMs,omitempty"`
	HasClock     bool   `json:"hasClock,omitempty"`
	// Draw state
	DrawOffered  bool   `json:"drawOffered,omitempty"` // true when opponent has offered a draw
	ClaimableDraw string `json:"claimableDraw,omitempty"` // non-empty notice when draw can be claimed
}

type LegalMove struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Promotion string `json:"promotion,omitempty"`
}

type ErrorMessage struct {
	Type    string `json:"type"`
	V       string `json:"v"`
	Message string `json:"message"`
}

type ThinkingMessage struct {
	Type string `json:"type"`
	V    string `json:"v"`
}

// SessionMessage is sent on connect to give the client its identity.
// The client should store Token in localStorage and pass it as ?token= on reconnect.
type SessionMessage struct {
	Type        string `json:"type"` // "session"
	V           string `json:"v"`
	PlayerID    string `json:"playerId"`
	DisplayName string `json:"displayName"`
	Token       string `json:"token"`
}

// RoomCreatedMessage is sent after create_room succeeds.
type RoomCreatedMessage struct {
	Type        string `json:"type"` // "room_created"
	V           string `json:"v"`
	RoomID      string `json:"roomId"`
	PlayerColor string `json:"playerColor"`
}

// RoomJoinedMessage is sent when a second player joins the room.
type RoomJoinedMessage struct {
	Type         string `json:"type"` // "room_joined"
	V            string `json:"v"`
	RoomID       string `json:"roomId"`
	PlayerColor  string `json:"playerColor"`
	OpponentName string `json:"opponentName"`
}

// RoomListMessage is sent in response to list_rooms.
type RoomListMessage struct {
	Type  string     `json:"type"` // "room_list"
	V     string     `json:"v"`
	Rooms []RoomInfo `json:"rooms"`
}

type RoomInfo struct {
	RoomID       string `json:"roomId"`
	Status       string `json:"status"` // "waiting" | "playing" | "finished"
	WhiteName    string `json:"whiteName"`
	BlackName    string `json:"blackName"`
	WhiteRating  int    `json:"whiteRating,omitempty"`
	BlackRating  int    `json:"blackRating,omitempty"`
	TimeControl  string `json:"timeControl,omitempty"`
	Spectators   int    `json:"spectators,omitempty"`
}

// OpponentDisconnectedMessage notifies the remaining player.
type OpponentDisconnectedMessage struct {
	Type string `json:"type"` // "opponent_disconnected"
	V    string `json:"v"`
}

// OpponentReconnectedMessage notifies when the opponent returns.
type OpponentReconnectedMessage struct {
	Type string `json:"type"` // "opponent_reconnected"
	V    string `json:"v"`
}

// DrawOfferedMessage is sent to the opponent when a draw is offered.
type DrawOfferedMessage struct {
	Type string `json:"type"` // "draw_offered"
	V    string `json:"v"`
}

// RatingUpdateMessage is sent to both players after a rated game ends.
type RatingUpdateMessage struct {
	Type      string `json:"type"` // "rating_update"
	V         string `json:"v"`
	OldRating int    `json:"oldRating"`
	NewRating int    `json:"newRating"`
	Delta     int    `json:"delta"`
}

// AuthOKMessage is sent after a successful register or login.
type AuthOKMessage struct {
	Type        string `json:"type"` // "auth_ok"
	V           string `json:"v"`
	Token       string `json:"token"`
	PlayerID    string `json:"playerId"`
	DisplayName string `json:"displayName"`
}

// MatchFoundMessage is sent to both players when matchmaking pairs them.
type MatchFoundMessage struct {
	Type         string `json:"type"` // "match_found"
	V            string `json:"v"`
	RoomID       string `json:"roomId"`
	PlayerColor  string `json:"playerColor"`
	OpponentName string `json:"opponentName"`
}

// MatchCancelledMessage confirms the player was removed from the queue.
type MatchCancelledMessage struct {
	Type string `json:"type"` // "match_cancelled"
	V    string `json:"v"`
}

// MatchWaitingMessage tells the client they are in the matchmaking queue.
type MatchWaitingMessage struct {
	Type        string `json:"type"` // "match_waiting"
	V           string `json:"v"`
	TimeControl string `json:"timeControl"`
}
