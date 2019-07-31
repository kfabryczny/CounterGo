package engine

import (
	. "github.com/ChizhovVadim/CounterGo/common"
)

const (
	stackSize     = 64
	maxHeight     = stackSize - 1
	valueDraw     = 0
	valueMate     = 30000
	valueInfinity = valueMate + 1
	valueWin      = valueMate - 2*maxHeight
	valueLoss     = -valueWin
)

func winIn(height int) int {
	return valueMate - height
}

func lossIn(height int) int {
	return -valueMate + height
}

func valueToTT(v, height int) int {
	if v >= valueWin {
		return v + height
	}

	if v <= valueLoss {
		return v - height
	}

	return v
}

func valueFromTT(v, height int) int {
	if v >= valueWin {
		return v - height
	}

	if v <= valueLoss {
		return v + height
	}

	return v
}

func newUciScore(v int) UciScore {
	if v >= valueWin {
		return UciScore{Mate: (valueMate - v + 1) / 2}
	} else if v <= valueLoss {
		return UciScore{Mate: (-valueMate - v) / 2}
	} else {
		return UciScore{Centipawns: v}
	}
}

func isLateEndgame(p *Position, side bool) bool {
	//sample: position fen 8/8/6p1/1p2pk1p/1Pp1p2P/2PbP1P1/3N1P2/4K3 w - - 12 58
	var ownPieces = p.PiecesByColor(side)
	return ((p.Rooks|p.Queens)&ownPieces) == 0 &&
		!MoreThanOne((p.Knights|p.Bishops)&ownPieces)
}

func isPawnEndgame(p *Position, side bool) bool {
	return ((p.Knights | p.Bishops | p.Rooks | p.Queens) &
		p.PiecesByColor(side)) == 0
}

var (
	pieceValues = [...]int{0, PawnValue, 4 * PawnValue,
		4 * PawnValue, 6 * PawnValue, 12 * PawnValue, 120 * PawnValue}

	pieceValuesSEE = [...]int{0, 1, 4, 4, 6, 12, 120}
)

func moveValue(move Move) int {
	var result = pieceValues[move.CapturedPiece()]
	if move.Promotion() != Empty {
		result += pieceValues[move.Promotion()] - pieceValues[Pawn]
	}
	return result
}

func isCaptureOrPromotion(move Move) bool {
	return move.CapturedPiece() != Empty ||
		move.Promotion() != Empty
}

func isDangerCapture(p *Position, m Move) bool {
	if m.CapturedPiece() == Pawn {
		var pawns = p.Pawns & p.PiecesByColor(!p.WhiteMove)
		if (pawns & (pawns - 1)) == 0 {
			return true
		}
	}
	return false
}

func isPawnPush7th(move Move, side bool) bool {
	if move.MovingPiece() != Pawn {
		return false
	}
	var rank = Rank(move.To())
	if side {
		return rank == Rank7
	} else {
		return rank == Rank2
	}
}

func isPawnAdvance(move Move, side bool) bool {
	if move.MovingPiece() != Pawn {
		return false
	}
	var rank = Rank(move.To())
	if side {
		return rank >= Rank6
	} else {
		return rank <= Rank3
	}
}

func getAttacks(p *Position, to int, side bool, occ uint64) uint64 {
	var att = (PawnAttacks(to, !side) & p.Pawns) |
		(KnightAttacks[to] & p.Knights) |
		(KingAttacks[to] & p.Kings) |
		(BishopAttacks(to, occ) & (p.Bishops | p.Queens)) |
		(RookAttacks(to, occ) & (p.Rooks | p.Queens))
	return p.PiecesByColor(side) & att
}

func getLeastValuableAttacker(p *Position, to int, side bool, occ uint64) (attacker, from int) {
	attacker = Empty
	from = SquareNone
	var att = getAttacks(p, to, side, occ) & occ
	if att == 0 {
		return
	}
	var newTarget = pieceValuesSEE[King] + 1
	for ; att != 0; att &= att - 1 {
		var f = FirstOne(att)
		var piece = p.WhatPiece(f)
		if pieceValuesSEE[piece] < newTarget {
			attacker = piece
			from = f
			newTarget = pieceValuesSEE[piece]
		}
	}
	return
}

func seeGEZero(p *Position, move Move) bool {
	return seeGE(p, move, 0)
}

func seeGE(p *Position, move Move, bound int) bool {
	var piece = move.MovingPiece()
	var score0 = pieceValuesSEE[move.CapturedPiece()]
	if promotion := move.Promotion(); promotion != Empty {
		piece = move.Promotion()
		score0 += pieceValuesSEE[promotion] - pieceValuesSEE[Pawn]
	}
	var to = move.To()
	var occ = p.White ^ p.Black ^ SquareMask[move.From()]
	occ |= SquareMask[to]
	var side = !p.WhiteMove
	var relativeStm = true
	var balance = score0 - bound
	if balance < 0 {
		return false
	}
	balance -= pieceValuesSEE[piece]
	if balance >= 0 {
		return true
	}
	for {
		var nextVictim, from = getLeastValuableAttacker(p, to, side, occ)
		if nextVictim == Empty {
			return relativeStm
		}
		if piece == King {
			return !relativeStm
		}
		occ ^= SquareMask[from]
		piece = nextVictim
		if relativeStm {
			balance += pieceValuesSEE[nextVictim]
		} else {
			balance -= pieceValuesSEE[nextVictim]
		}
		relativeStm = !relativeStm
		if relativeStm == (balance >= 0) {
			return relativeStm
		}
		side = !side
	}
}

func see(pos *Position, mv Move) int {
	var from = mv.From()
	var to = mv.To()
	var pc = mv.MovingPiece()
	var sd = pos.WhiteMove
	var sc = 0
	if mv.CapturedPiece() != Empty {
		sc += pieceValuesSEE[mv.CapturedPiece()]
	}
	if mv.Promotion() != Empty {
		pc = mv.Promotion()
		sc += pieceValuesSEE[pc] - pieceValuesSEE[Pawn]
	}
	var pieces = (pos.White | pos.Black) &^ SquareMask[from]
	sc -= seeRec(pos, !sd, to, pieces, pc)
	return sc
}

func seeRec(pos *Position, sd bool, to int, pieces uint64, cp int) int {
	var bs = 0
	var pc, from = getLeastValuableAttacker(pos, to, sd, pieces)
	if from != SquareNone {
		var sc = pieceValuesSEE[cp]
		if cp != King {
			sc -= seeRec(pos, !sd, to, pieces&^SquareMask[from], pc)
		}
		if sc > bs {
			bs = sc
		}
	}
	return bs
}

func lmrByMoveIndex(d, m int) int {
	if m > 22 {
		return 3
	}
	if m > 8 {
		return 2
	}
	return 1
}

func lmrMain(d, m int) int {
	var r = 1
	if m > 6 {
		r += (2*d + m) / 16
	}
	return r
}

func evaluateMaterial(p *Position) int {
	var score = 100*(PopCount(p.Pawns&p.White)-PopCount(p.Pawns&p.Black)) +
		400*(PopCount((p.Knights|p.Bishops)&p.White)-PopCount((p.Knights|p.Bishops)&p.Black)) +
		600*(PopCount(p.Rooks&p.White)-PopCount(p.Rooks&p.Black)) +
		1200*(PopCount(p.Queens&p.White)-PopCount(p.Queens&p.Black))
	if !p.WhiteMove {
		score = -score
	}
	return score
}
