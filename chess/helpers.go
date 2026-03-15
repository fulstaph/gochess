package chess

func inBounds(r, c int) bool {
	return r >= 0 && r < 8 && c >= 0 && c < 8
}

func colorOf(piece rune) int {
	if piece >= 'A' && piece <= 'Z' {
		return White
	}
	if piece >= 'a' && piece <= 'z' {
		return Black
	}
	return 0
}

func pieceType(piece rune) rune {
	if piece >= 'A' && piece <= 'Z' {
		return piece + ('a' - 'A')
	}
	return piece
}

func promotionPieces(color int) []rune {
	if color == White {
		return []rune{'Q', 'R', 'B', 'N'}
	}
	return []rune{'q', 'r', 'b', 'n'}
}

func ColorName(color int) string {
	if color == White {
		return "White"
	}
	return "Black"
}
