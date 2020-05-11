package message

import (
	"encoding/binary"
	"fmt"
	"io"
)

type messageID uint8

const (
	MsgChoke messageID = 0
	MsgUnchoke messageID = 1
	MsgInterested messageID = 2
	MsgNotInterested messageID = 3
	MsgHave messageID = 4
	MsgBitfield messageID = 5
	MsgRequest messageID = 6
	MsgPiece messageID = 7
	MsgCancel messageID = 8
)

type Message struct {
	ID messageID
	Payload []byte
}

func (m *Message) name() string {
	if m == nil {
		return "KeepAlive"
	}

	switch m.ID {
	case MsgChoke:
		return "Choke"
	case MsgUnchoke:
		return "Unchoke"
	case MsgInterested:
		return "Interested"
	case MsgNotInterested:
		return "NotInterested"
	case MsgHave:
		return "Have"
	case MsgBitfield:
		return "Bitfield"
	case MsgRequest:
		return "Request"
	case MsgPiece:
		return "Piece"
	case MsgCancel:
		return "Cancel"
	default:
		return fmt.Sprintf("Unknown#%d", m.ID)
	}
}

// Serialize returns a buffer for the message
// The first four bytes are the length of the message
// The fifth byte is the message ID
// All bytes after that are the payload
func (m *Message) Serialize() []byte {
	if m == nil {
		return make([]byte, 4)
	}

	length := uint32(len(m.Payload) + 1) // Add a byte for the message id
	buf := make([]byte, 4 + length)
	binary.BigEndian.PutUint32(buf[0:4], length)
	buf[4] = byte(m.ID)
	copy(buf[5:], m.Payload)
	return buf
}

func (m *Message) String() string {
	// In the nil case, we'll print KeepAlive
	if m == nil {
		return m.name()
	}

	return fmt.Sprintf("%s [%d]", m.name(), len(m.Payload))
}

// FormatRequest returns a Message struct that contains an ID and Payload
// ID is always MsgRequest (or 6)
// Payload is a 12 byte buffer consisting of
//		4 bytes of the index
//		4 bytes of the begin offset
// 		4 bytes of the length of block we'd like to receive
func FormatRequest(index, begin, length int) *Message {
	// NOTE: Why 12?
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], uint32(index))
	binary.BigEndian.PutUint32(payload[4:8], uint32(begin))
	binary.BigEndian.PutUint32(payload[8:12], uint32(length))

	return &Message{ID: MsgRequest, Payload: payload}
}

func FormatHave(index int) *Message {
	payload := make([]byte, 4)

	binary.BigEndian.PutUint32(payload, uint32(index))
	return &Message{
		ID: MsgHave,
		Payload: payload,
	}
}


func Read(r io.Reader) (*Message, error) {
	lengthBuf := make([]byte, 4)

	// Reads four bytes into the buffer?
	_, err := io.ReadFull(r, lengthBuf)

	if err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lengthBuf)

	// This case is for keep-alive messages
	if length == 0 {
		return nil, nil
	}

	messageBuf := make([]byte, length)
	_, err = io.ReadFull(r, messageBuf)

	if err != nil {
		return nil, err
	}

	return &Message {
		ID: messageID(messageBuf[0]),
		Payload: messageBuf[1:],
	}, nil
}

func ParseHave(msg *Message) (int, error) {
	if msg.ID != MsgHave {
		return 0, fmt.Errorf("Expected HAVE (ID %d), got ID %d", MsgHave, msg.ID)
	}

	if len(msg.Payload) != 4 {
		return 0, fmt.Errorf("Expected payload length 4, got length %d", len(msg.Payload))
	}

	index := int(binary.BigEndian.Uint32(msg.Payload))
	return index, nil
}

func ParsePiece(index int, buf []byte, msg *Message) (int, error) {
	if msg.ID != MsgPiece {
		return 0, fmt.Errorf("Expected PIECE (ID %d), but got ID %d", MsgPiece, msg.ID)
	}

	if len(msg.Payload) < 8 {
		return 0, fmt.Errorf("Payload too short, expected 8 or more but got %d", len(msg.Payload))
	}

	parsedIndex := int(binary.BigEndian.Uint32(msg.Payload[0:4]))
	if parsedIndex != index {
		return 0, fmt.Errorf("Expected index %d, but got %d", index, parsedIndex)
	}

	begin := int(binary.BigEndian.Uint32(msg.Payload[4:8]))

	if begin >= len(buf) {
		return 0, fmt.Errorf("Begin index is too high. %d is bigger than %d", begin, len(buf))
	}

	data := msg.Payload[8:]
	if begin + len(data) > len(buf) {
		return 0, fmt.Errorf("Data length is too large. %d is bigger than %d", begin + len(data), len(buf))
	}

	copy(buf[begin:], data)

	return len(data), nil
}
