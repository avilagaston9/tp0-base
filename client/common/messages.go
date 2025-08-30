package common

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"
)

const maxAllowedBatchSize = 8000

// name and surname + document + birthdate + number + agency
const MaxSerializedBetSize = 514 + 4 + 10 + 4 + 1

const DefaultMaxBatchAmount = (maxAllowedBatchSize - 1 /*(MsgId)*/ - 1 /*(MsgType)*/ - 2 /*(BetsCount)*/) / MaxSerializedBetSize

type MessageType uint8

const (
	TypeBet MessageType = iota
	TypeResult
	TypeBatch
)

type Bet struct {
	Name      string
	Surname   string
	Document  uint32
	Birthdate string
	Number    uint32
	Agency    uint8
}

type Batch struct {
	Bets []*Bet
}

type Result struct {
	MsjID   uint8
	Success bool
}

func NewBet(name string, surname string, doc string, birthdate string, num string, agency uint8) (*Bet, error) {
	if err := ValidateName(name, surname); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	_, err := time.Parse("2006-01-02", birthdate)
	if err != nil {
		return nil, fmt.Errorf("unable to parse birthdate: %w", err)
	}
	document, err := strconv.ParseUint(doc, 10, 32)
	if err != nil {
		return nil, errors.New("document must be numeric")
	}
	number, err := strconv.ParseUint(num, 10, 32)
	if err != nil {
		return nil, errors.New("lottery number must be numeric")
	}

	bet := &Bet{
		Name:      name,
		Surname:   surname,
		Document:  uint32(document),
		Birthdate: birthdate,
		Number:    uint32(number),
		Agency:    agency,
	}
	return bet, nil
}

func ValidateName(name string, surname string) error {
	if name == "" || surname == "" {
		return errors.New("name and surname are required")
	}

	if len(name) > math.MaxUint8 {
		return errors.New("name too long")
	}

	if len(surname) > math.MaxUint8 {
		return errors.New("surname too long")
	}

	return nil
}

func (b *Bet) ToMessageBytes(msgId uint8) []byte {
	buf := new(bytes.Buffer)

	// MsgID (1 byte)
	binary.Write(buf, binary.BigEndian, msgId)

	// Message type
	binary.Write(buf, binary.BigEndian, TypeBet)

	b.encode(buf)
	return buf.Bytes()
}

func (b *Bet) encode(buf *bytes.Buffer) {
	// Name and Surname (1 bytes length + variable length)
	encodeString(buf, b.Name)
	encodeString(buf, b.Surname)

	// Document (4 bytes fixed length)
	binary.Write(buf, binary.BigEndian, b.Document)

	// Birthdate (10 bytes - Unix timestamp)
	buf.WriteString(b.Birthdate)
	// Lottery number (4 bytes fixed length)
	binary.Write(buf, binary.BigEndian, b.Number)

	// Agency (1 byte)
	binary.Write(buf, binary.BigEndian, b.Agency)

}

func encodeString(buf *bytes.Buffer, s string) {
	binary.Write(buf, binary.BigEndian, uint8(len(s)))
	buf.WriteString(s)
}

func (b *Batch) ToMessageBytes(msgId uint8) []byte {
	buf := new(bytes.Buffer)

	// MsgID (1 byte)
	binary.Write(buf, binary.BigEndian, msgId)

	// Message type
	binary.Write(buf, binary.BigEndian, TypeBatch)

	b.encode(buf)

	return buf.Bytes()

}

func (b *Batch) encode(buf *bytes.Buffer) {
	// Number of bets (2 bytes)
	binary.Write(buf, binary.BigEndian, uint16(len(b.Bets)))

	for _, bet := range b.Bets {
		bet.encode(buf)
	}
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
