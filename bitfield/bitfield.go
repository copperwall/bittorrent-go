package bitfield

type Bitfield []byte

// HasPiece is a predicate for if a Bitfield 
func (bf Bitfield) HasPiece(index int) bool {
	byteIndex := index / 8
	offset := index % 8

	if byteIndex < 0 || byteIndex >= len(bf) {
		return false
	}

	return bf[byteIndex] >> (7 - offset)&1 != 0
}

// SetPiece sets a piece in the bitfield
// This isn't a pointer because bf is already
// a byte slice.
func (bf Bitfield) SetPiece(index int) {
	byteIndex := index / 8
	offset := index % 8

	if byteIndex < 0 || byteIndex >= len(bf) {
		return
	}

	bf[byteIndex] |= 1 << (7 - offset)
}
