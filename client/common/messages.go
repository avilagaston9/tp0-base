package common

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"time"
)

type MessageType uint8

const (
	TypeBet MessageType = iota
	TypeResult
)

type Bet struct {
	Name      string
	Surname   string
	Document  string
	Birthdate string
	Number    string
	MsgId     uint8
	Agency    uint8
}

func NewBet(name string, surname string, document string, birthdate string, number string, msgId uint8, agency uint8) (*Bet, error) {
	if name == "" || surname == "" {
		return nil, errors.New("name and surname are required")
	}
	if err := ValidateDocument(document); err != nil {
		return nil, fmt.Errorf("invalid document: %w", err)
	}
	_, err := time.Parse("2006-01-02", birthdate)
	if err != nil {
		return nil, fmt.Errorf("unable to parse birthdate: %w", err)
	}
	if err := ValidateNumber(number); err != nil {
		return nil, fmt.Errorf("invalid lottery number: %w", err)
	}
	bet := &Bet{
		Name:      name,
		Surname:   surname,
		Document:  document,
		Birthdate: birthdate,
		Number:    number,
		MsgId:     msgId,
		Agency:    agency,
	}
	return bet, nil
}

func (b *Bet) Encode() []byte {
	buf := new(bytes.Buffer)

	// Message type
	binary.Write(buf, binary.BigEndian, TypeBet)

	// // Payload size
	// payloadSize := uint16(4 + 8 + 4)
	// payloadSize += 2 + uint16(len(b.Name))
	// payloadSize += 2 + uint16(len(b.Surname))

	encodeString(buf, b.Name)
	encodeString(buf, b.Surname)

	// Document (8 bytes fixed length)
	buf.WriteString(string(b.Document))

	// Birthdate (10 bytes - Unix timestamp)
	buf.WriteString(string(b.Birthdate))

	// Lottery number (4 bytes fixed length)
	buf.WriteString(string(b.Number))

	binary.Write(buf, binary.BigEndian, b.MsgId)
	binary.Write(buf, binary.BigEndian, b.Agency)

	return buf.Bytes()
}

func ValidateDocument(d string) error {
	matched, _ := regexp.MatchString(`^\d{8}$`, string(d))
	if !matched {
		return errors.New("invalid document format")
	}
	return nil
}

func ValidateNumber(n string) error {
	matched, _ := regexp.MatchString(`^\d{4}$`, string(n))
	if !matched {
		return errors.New("lottery number must be 4 digits")
	}
	return nil
}

func encodeString(buf *bytes.Buffer, s string) {
	binary.Write(buf, binary.BigEndian, uint16(len(s)))
	buf.WriteString(s)
}

type Result struct {
	MsjID   uint8
	Success bool
}

func DecodeResult(data []byte) (*Result, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("message too short: expected 4 bytes, got %d", len(data))
	}

	buf := bytes.NewReader(data)

	// Read message type (1 byte)
	var msgType MessageType
	if err := binary.Read(buf, binary.BigEndian, &msgType); err != nil {
		return nil, fmt.Errorf("error reading message type: %w", err)
	}
	if msgType != TypeResult {
		return nil, fmt.Errorf("invalid message type: expected %d, got %d", TypeResult, msgType)
	}

	// Read MsgID (1 bytes)
	var msgID uint8
	if err := binary.Read(buf, binary.BigEndian, &msgID); err != nil {
		return nil, fmt.Errorf("error reading message ID: %w", err)
	}

	// Read Result (1 byte)
	var resultByte uint8
	if err := binary.Read(buf, binary.BigEndian, &resultByte); err != nil {
		return nil, fmt.Errorf("error reading result: %w", err)
	}
	result := resultByte != 0

	return &Result{
		MsjID:   msgID,
		Success: result,
	}, nil
}

func ReadResultMessage(conn net.Conn) (*Result, error) {
	buf := make([]byte, 3)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, fmt.Errorf("error reading message: %w", err)
	}

	// Decode the message
	return DecodeResult(buf)
}
