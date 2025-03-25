package common

import (
	"fmt"
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
	config           ClientConfig
	conn             net.Conn
	gracefulShutdown chan struct{}
	bets             []*Bet
	agency           uint8
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig) (*Client, error) {
	agency, err := strconv.ParseUint(config.ID, 10, 8)
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %w", err)
	}
	client := &Client{
		config:           config,
		gracefulShutdown: make(chan struct{}),
		agency:           uint8(agency),
	}
	bets, err := client.getBets()
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %w", err)
	}
	client.bets = bets
	go func() {
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, syscall.SIGTERM)
		<-sigchan
		log.Infof("action: graceful_shutdown | result: in_progress | client_id: %v", config.ID)
		close(client.gracefulShutdown)
	}()
	return client, nil
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
	conn.SetDeadline(time.Now().Add(Timeout))
	c.conn = conn
	return nil
}

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop() {
	for _, bet := range c.bets {
		select {
		case <-c.gracefulShutdown:
			log.Infof("action: graceful_shutdown | result: success | client_id: %v", c.config.ID)
			return
		default:
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
			r, err := ReadResultMessage(c.conn)

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

func (c *Client) getBets() ([]*Bet, error) {
	name := os.Getenv("NOMBRE")
	surname := os.Getenv("APELLIDO")
	document := os.Getenv("DOCUMENTO")
	birthdate := os.Getenv("NACIMIENTO")
	number := os.Getenv("NUMERO")

	bet, err := NewBet(name, surname, document, birthdate, number, 1, c.agency)
	if err != nil {
		return nil, err
	}

	return []*Bet{bet}, nil
}
