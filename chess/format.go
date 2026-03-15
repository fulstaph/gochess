package chess

// FormatMove renders a move in coordinate notation, using O-O / O-O-O for castling.
func FormatMove(mv Move) string {
	if mv.isCastle {
		if mv.toC == 6 {
			return "O-O"
		}
		return "O-O-O"
	}

	from := formatSquare(mv.fromR, mv.fromC)
	to := formatSquare(mv.toR, mv.toC)
	if mv.promotion != 0 {
		promo := pieceType(mv.promotion)
		return from + to + string(promo)
	}
	return from + to
}

func formatSquare(r, c int) string {
	file := byte('a' + c)
	rank := byte('8' - r)
	return string([]byte{file, rank})
}

func (mv Move) From() (int, int) {
	return mv.fromR, mv.fromC
}

func (mv Move) To() (int, int) {
	return mv.toR, mv.toC
}
