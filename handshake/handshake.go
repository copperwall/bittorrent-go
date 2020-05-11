package handshake

import (
	"fmt"
	"io"
)

const InfoHashLength = 20
const PeerIDLength = 20
const ReservedByteLength = 8

type Handshake struct {
	Pstr string
	InfoHash [InfoHashLength]byte
	PeerID [PeerIDLength]byte
}

// Serialize returns a buffer for the Handshake instance
// The first byte is the length of the pstr
// 
func (h *Handshake) Serialize() []byte {
	pstrLen := len(h.Pstr)
	bufLen := 49 + pstrLen

	// make a buf with the pstrLength + 49
	buf := make([]byte, bufLen)

	buf[0] = byte(len(h.Pstr))

	curr := 1

	// Protocol Standard
	curr += copy(buf[curr:], h.Pstr)
	// Reserved bytes
	curr += copy(buf[curr:], make([]byte, 8))
	// InfoHash
	curr += copy(buf[curr:], h.InfoHash[:])
	// PeerID
	curr += copy(buf[curr:], h.PeerID[:])

	return buf
}

func Read(r io.Reader) (*Handshake, error) {
	fmt.Println("Make it to handshake read")
	lengthBuf := make([]byte, 1)
	_, err := io.ReadFull(r, lengthBuf)

	fmt.Println("Read into the length buf")

	if err != nil {
		return nil, err
	}

	// What's pstr again?
	// The protocol identifier, which should always be "BitTorrent Protocol"
	pstrlen := int(lengthBuf[0])

	if pstrlen == 0 {
		err := fmt.Errorf("pstrlen cannot be 0")
		return nil, err
	}

	handshakeBuf := make([]byte, ReservedByteLength + InfoHashLength + PeerIDLength + pstrlen)
	_, err = io.ReadFull(r, handshakeBuf)

	if err != nil {
		return nil, err
	}

	// PeerID and InfoHash have the same length
	var infoHash, peerID [InfoHashLength]byte

	copy(infoHash[:], handshakeBuf[pstrlen + ReservedByteLength : pstrlen + ReservedByteLength + InfoHashLength])
	copy(peerID[:], handshakeBuf[pstrlen + ReservedByteLength + InfoHashLength:])

	h := Handshake {
		Pstr: string(handshakeBuf[0 : pstrlen]),
		InfoHash: infoHash,
		PeerID: peerID,
	}

	return &h, nil
}

// New creates a new Handshake struct
func New(infoHash, peerID [PeerIDLength]byte) *Handshake {
	return &Handshake{
		Pstr: "BitTorrent protocol",
		InfoHash: infoHash,
		PeerID: peerID,
	}
}
