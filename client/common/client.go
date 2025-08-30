package common

import (
	"context"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

const Timeout = 500 * time.Millisecond

// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID             string
	ServerAddress  string
	LoopAmount     int
	LoopPeriod     time.Duration
	BatchMaxAmount int
}

// Client Entity that encapsulates how
type Client struct {
	config  ClientConfig
	conn    net.Conn
	ctx     context.Context
	cancel  context.CancelFunc
	agency  uint8
	batches []*Batch
}

func getBets(agency uint8) ([]*Bet, error) {
	filePath := fmt.Sprintf(".data/agency-%d.csv", agency)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Read the CSV
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	// Parse each row into a Bet
	var bets []*Bet
	for _, record := range records {
		if len(record) != 5 {
			return nil, fmt.Errorf("invalid record length: %v", record)
		}
		name, surname, document, birthdate, number := record[0], record[1], record[2], record[3], record[4]

		bet, err := NewBet(name, surname, document, birthdate, number, agency)
		if err != nil {
			return nil, fmt.Errorf("failed to create bet: %w", err)
		}
		bets = append(bets, bet)
	}

	return bets, nil
}

func getBatches(agency uint8, batchMaxAmount int) ([]*Batch, error) {
	bets, err := getBets(agency)
	if err != nil {
		return nil, err
	}

	var batches []*Batch
	for i := 0; i < len(bets); i += batchMaxAmount {
		end := i + batchMaxAmount
		if end > len(bets) {
			end = len(bets)
		}

		batch := &Batch{
			Bets: bets[i:end],
		}
		batches = append(batches, batch)
	}

	return batches, nil

}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig) (*Client, error) {
	agency, err := strconv.ParseUint(config.ID, 10, 8)
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %w", err)
	}

	batches, err := getBatches(uint8(agency), config.BatchMaxAmount)
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	return &Client{
		config:  config,
		ctx:     ctx,
		cancel:  stop,
		batches: batches,
		agency:  uint8(agency),
	}, nil
}

// CreateClientSocket Initializes client socket. In case of
// failure, error is printed in stdout/stderr and exit 1
// is returned
func (c *Client) createClientSocket() error {
	conn, err := net.DialTimeout("tcp", c.config.ServerAddress, Timeout)
	if err != nil {
		log.Criticalf(
			"action: connect | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
		return fmt.Errorf("unable to create socket: %w", err)
	}
	c.conn = conn
	return nil
}

func (c *Client) StartClientLoop() {
	for i, batch := range c.batches {
		select {
		case <-c.ctx.Done():
			log.Infof("action: graceful_shutdown | result: success | client_id: %v", c.config.ID)
			return
		default:
			err := c.createClientSocket()
			if err != nil {
				log.Errorf("action: apuesta_enviada | result: fail | agency: %v | batch_number: %v",
					c.agency,
					i,
				)
				return
			}

			err = c.sendMessage(batch.ToMessageBytes(uint8(rand.Intn(256))))
			if err != nil {
				log.Errorf("action: apuesta_enviada | result: fail | agency: %v | batch_number: %v",
					c.agency,
					i,
				)
				return
			}
			r, err := c.readResultMessage()

			if err != nil {
				log.Errorf("action: apuesta_enviada | result: fail | agency: %v | batch_number: %v",
					c.agency,
					i,
				)
				log.Errorf("error: %v ", err)
				return
			}
			c.conn.Close()
			if r.Success {
				log.Errorf("action: apuesta_enviada | result: success | agency: %v | batch_number: %v",
					c.agency,
					i,
				)
			}
		}
	}
	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)

	c.requestWinners()

	// Sleeping to let the container print the exit log
	time.Sleep(100 * time.Millisecond)
}

func (c *Client) sendMessage(data []byte) error {
	totalWritten := 0
	for totalWritten < len(data) {
		n, err := c.conn.Write(data[totalWritten:])
		if err != nil {
			return fmt.Errorf("short write: %w", err)
		}
		totalWritten += n
	}
	return nil
}

func (c *Client) readResultMessage() (*Result, error) {
	buf := make([]byte, 3)
	c.conn.SetReadDeadline(time.Now().Add(Timeout))
	if _, err := io.ReadFull(c.conn, buf); err != nil {
		return nil, fmt.Errorf("error reading message: %w", err)
	}

	// Decode the message
	return DecodeResult(buf)
}

func (c *Client) requestWinners() {
	finished := NewFinished(c.agency)

	notReady := true
	var winners *Winners

	for notReady {
		select {
		case <-c.ctx.Done():
			log.Infof("action: graceful_shutdown | result: success | client_id: %v", c.config.ID)
			return
		default:
			err := c.createClientSocket()
			if err != nil {
				log.Errorf("action: envio_fin | result: fail | agency: %v | error: %v",
					c.agency,
					err,
				)
				return
			}
			err = c.sendMessage(finished.ToMessageBytes(uint8(rand.Intn(256))))
			if err != nil {
				log.Errorf("action: envio_fin | result: fail | agency: %v | error: %v",
					c.agency,
					err,
				)
				return
			}
			notReady, winners, err = c.readWinnersResponse()
			if err != nil {
				log.Errorf("action: consulta_ganadores | result: fail | agency: %v | error: %v",
					c.agency,
					err,
				)
				c.conn.Close()
				return
			}
			c.conn.Close()
		}
	}
	log.Infof("action: consulta_ganadores | result: success  | cant_ganadores: %v",
		len(winners.Documents),
	)
}

func (c *Client) readWinnersResponse() (bool, *Winners, error) {
	// Read message type (1 byte)
	var msgType MessageType
	c.conn.SetReadDeadline(time.Now().Add(Timeout))
	if err := binary.Read(c.conn, binary.BigEndian, &msgType); err != nil {
		return false, nil, fmt.Errorf("error reading message type: %w", err)
	}

	switch msgType {
	case TypeWinners:
		w, err := c.readWinnersMessage()
		if err != nil {
			return false, nil, err
		}
		return false, w, nil

	case TypeNotReady:
		return true, nil, nil

	default:
		return true, nil, fmt.Errorf("invalid message type: expected %d or %d, got %d", TypeWinners, TypeNotReady, msgType)
	}
}

func (c *Client) readWinnersMessage() (*Winners, error) {
	// Read winners_count (uint16)
	c.conn.SetReadDeadline(time.Now().Add(Timeout))
	var countBuf [2]byte
	if _, err := io.ReadFull(c.conn, countBuf[:]); err != nil {
		return nil, fmt.Errorf("error reading winners count: %w", err)
	}
	winners_count := binary.BigEndian.Uint16(countBuf[:])

	// Read `count` documents
	winners := &Winners{
		Documents: make([]uint32, 0, winners_count),
	}

	var docBuf [4]byte
	for i := 0; i < int(winners_count); i++ {
		if _, err := io.ReadFull(c.conn, docBuf[:]); err != nil {
			return nil, fmt.Errorf("error reading document %d: %w", i, err)
		}
		doc := binary.BigEndian.Uint32(docBuf[:])
		winners.Documents = append(winners.Documents, doc)
	}

	return winners, nil
}
