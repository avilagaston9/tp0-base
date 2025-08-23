package common

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
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

type Result struct {
	MsjID   uint8
	Success bool
}

func NewBet(name string, surname string, document string, birthdate string, number string, msgId uint8, agency uint8) (*Bet, error) {
	if err := ValidateName(name, surname); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
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

func ValidateName(name string, surname string) error {
	if name == "" || surname == "" {
		return errors.New("name and surname are required")
	}

	if len(name) > math.MaxUint16 {
		return errors.New("name too long")
	}

	if len(surname) > math.MaxUint16 {
		return errors.New("surname too long")
	}

	return nil
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

func (b *Bet) Encode() []byte {
	buf := new(bytes.Buffer)

	// Message type
	binary.Write(buf, binary.BigEndian, TypeBet)

	// Name and Surname (2 bytes length + variable length)
	encodeString(buf, b.Name)
	encodeString(buf, b.Surname)

	// Document (8 bytes fixed length)
	buf.WriteString(string(b.Document))

	// Birthdate (10 bytes - Unix timestamp)
	buf.WriteString(string(b.Birthdate))

	// Lottery number (4 bytes fixed length)
	buf.WriteString(string(b.Number))

	// MsgID (1 byte) and Agency (1 byte)
	binary.Write(buf, binary.BigEndian, b.MsgId)
	binary.Write(buf, binary.BigEndian, b.Agency)

	return buf.Bytes()
}

func encodeString(buf *bytes.Buffer, s string) {
	binary.Write(buf, binary.BigEndian, uint16(len(s)))
	buf.WriteString(s)
}

func DecodeResult(data []byte) (*Result, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("message too short: expected 3 bytes, got %d", len(data))
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

	// Read Success (1 byte)
	var successByte uint8
	if err := binary.Read(buf, binary.BigEndian, &successByte); err != nil {
		return nil, fmt.Errorf("error reading result: %w", err)
	}
	success := successByte != 0

	return &Result{
		MsjID:   msgID,
		Success: success,
	}, nil
}
