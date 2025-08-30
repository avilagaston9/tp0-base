package common

import (
	"context"
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
	config    ClientConfig
	conn      net.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	agency    uint8
	csvFile   *os.File
	csvReader *csv.Reader
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig) (*Client, error) {
	agency, err := strconv.ParseUint(config.ID, 10, 8)
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %w", err)
	}

	filePath := fmt.Sprintf(".data/agency-%d.csv", uint8(agency))
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)

	return &Client{
		config:    config,
		ctx:       ctx,
		cancel:    stop,
		agency:    uint8(agency),
		csvFile:   file,
		csvReader: csv.NewReader(file),
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
	defer c.csvFile.Close()

	batchNum := 0
	for {
		select {
		case <-c.ctx.Done():
			log.Infof("action: graceful_shutdown | result: success | client_id: %v", c.config.ID)
			return
		default:
			batch, err := c.fetchNextBatch()
			if err == io.EOF {
				log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
				time.Sleep(100 * time.Millisecond) // let container log
				return
			}
			if err != nil {
				log.Errorf("action: fetch_batch | result: fail | error: %v", err)
				return
			}

			err = c.createClientSocket()
			if err != nil {
				log.Errorf("action: apuesta_enviada | result: fail | agency: %v | batch_number: %v",
					c.agency, batchNum)
				return
			}

			err = c.sendMessage(batch.ToMessageBytes(uint8(rand.Intn(256))))
			if err != nil {
				c.conn.Close()
				log.Errorf("action: apuesta_enviada | result: fail | agency: %v | batch_number: %v",
					c.agency, batchNum)
				return
			}

			r, err := c.readResultMessage()
			c.conn.Close()
			if err != nil {
				log.Errorf("action: apuesta_enviada | result: fail | agency: %v | batch_number: %v",
					c.agency, batchNum)
				log.Errorf("error: %v", err)
				return
			}
			if r.Success {
				log.Infof("action: apuesta_enviada | result: success | agency: %v | batch_number: %v",
					c.agency, batchNum)
			}
			batchNum++
		}
	}
}

// fetchNextBatch reads bets until reaching batchMaxAmount or EOF
func (c *Client) fetchNextBatch() (*Batch, error) {
	var bets []*Bet
	for len(bets) < c.config.BatchMaxAmount {
		record, err := c.csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV: %w", err)
		}
		if len(record) != 5 {
			return nil, fmt.Errorf("invalid record length: %v", record)
		}

		bet, err := NewBet(record[0], record[1], record[2], record[3], record[4], c.agency)
		if err != nil {
			return nil, fmt.Errorf("failed to create bet: %w", err)
		}
		bets = append(bets, bet)
	}

	if len(bets) == 0 {
		return nil, io.EOF
	}

	return &Batch{Bets: bets}, nil
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
