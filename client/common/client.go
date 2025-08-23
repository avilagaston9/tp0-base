package common

import (
	"context"
	"fmt"
	"io"
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
	ID            string
	ServerAddress string
	LoopAmount    int
	LoopPeriod    time.Duration
}

// Client Entity that encapsulates how
type Client struct {
	config ClientConfig
	conn   net.Conn
	ctx    context.Context
	cancel context.CancelFunc
	bets   []*Bet
	agency uint8
}

func getBets(agency uint8) ([]*Bet, error) {
	name := os.Getenv("NOMBRE")
	surname := os.Getenv("APELLIDO")
	document := os.Getenv("DOCUMENTO")
	birthdate := os.Getenv("NACIMIENTO")
	number := os.Getenv("NUMERO")

	bet, err := NewBet(name, surname, document, birthdate, number, 1, agency)
	if err != nil {
		return nil, err
	}

	return []*Bet{bet}, nil
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig) (*Client, error) {
	agency, err := strconv.ParseUint(config.ID, 10, 8)
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %w", err)
	}

	bets, err := getBets(uint8(agency))
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	return &Client{
		config: config,
		ctx:    ctx,
		cancel: stop,
		bets:   bets,
		agency: uint8(agency),
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

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop() {
	for _, bet := range c.bets {
		select {
		case <-c.ctx.Done():
			log.Infof("action: graceful_shutdown | result: success | client_id: %v", c.config.ID)
			return
		default:
			// There is an autoincremental msgID to identify every message sent
			// Messages if the message amount threshold has not been surpassed
			err := c.createClientSocket()
			if err != nil {
				log.Errorf("action: apuesta_enviada | result: fail |  dni: %v | numero: %v",
					bet.Document,
					bet.Number,
				)
				return
			}

			err = c.sendMessage(bet.Encode())
			if err != nil {
				log.Errorf("action: apuesta_enviada | result: fail |  dni: %v | numero: %v",
					bet.Document,
					bet.Number,
				)
				return
			}
			r, err := c.readResultMessage()

			if err != nil {
				log.Errorf("action: apuesta_enviada | result: fail |  dni: %v | numero: %v",
					bet.Document,
					bet.Number,
				)
				log.Errorf("error: %v ", err)
				return
			}
			c.conn.Close()
			if r.Success {
				log.Infof("action: apuesta_enviada | result: success |  dni: %v | numero: %v",
					bet.Document,
					bet.Number,
				)

			}

			// Wait a time between sending one message and the next one
			time.Sleep(c.config.LoopPeriod)
		}
		log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
	}
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
