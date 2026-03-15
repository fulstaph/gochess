package chess

const (
	White = 1
	Black = -1
)

type GameState struct {
	board       [8][8]rune
	turn        int
	castling    [4]bool
	enPassantR  int
	enPassantC  int
	halfmove    int
	fullmoveNum int
}

type Move struct {
	fromR       int
	fromC       int
	toR         int
	toC         int
	promotion   rune
	isCastle    bool
	isEnPassant bool
}

func InitialState() GameState {
	var state GameState
	state.board = [8][8]rune{
		{'r', 'n', 'b', 'q', 'k', 'b', 'n', 'r'},
		{'p', 'p', 'p', 'p', 'p', 'p', 'p', 'p'},
		{0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0},
		{'P', 'P', 'P', 'P', 'P', 'P', 'P', 'P'},
		{'R', 'N', 'B', 'Q', 'K', 'B', 'N', 'R'},
	}
	state.turn = White
	state.castling = [4]bool{true, true, true, true}
	state.enPassantR = -1
	state.enPassantC = -1
	state.fullmoveNum = 1
	return state
}

func (s GameState) Board() [8][8]rune {
	return s.board
}

func (s GameState) Turn() int {
	return s.turn
}

func (s GameState) HalfmoveClock() int {
	return s.halfmove
}

func (s GameState) FullmoveNumber() int {
	return s.fullmoveNum
}
