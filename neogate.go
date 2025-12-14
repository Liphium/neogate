package neogate

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

type None struct{}

var DebugLogs = true
var Log = log.New(log.Writer(), "neogate ", log.Flags())

type Instance[T any] struct {
	Config           Config[T]
	connectionsCache *sync.Map // ID:Session -> Client
	sessionsCache    *sync.Map // ID -> Session list
	adapters         *sync.Map // ID -> Adapter
	routes           map[string]func(*Context[T]) Event
}

type ClientInfo[T any] struct {
	ID      string // Identifier of the account
	Session string // Identifier of the session
	Data    T      // Session data you can decide how to fill
}

// Convert the client information to a client that can be used by neogate.
func (info ClientInfo[T]) ToClient(conn *websocket.Conn) Client[T] {
	return Client[T]{
		Conn:    conn,
		ID:      info.ID,
		Session: info.Session,
		Data:    info.Data,
		Mutex:   &sync.Mutex{},
	}
}

// ! If the functions aren't implemented pipesfiber will panic
// The generic should be the type of the handshake request
type Config[T any] struct {

	// Config options
	HandshakeTimeout time.Duration // Timeout for handshake message

	// Called when a client attempts to connection. Return true if the connection is allowed.
	// MUST BE SPECIFIED.
	Handshake func(data T) (ClientInfo[T], bool)

	// Client handlers
	ClientDisconnectHandler   func(client *Client[T])
	ClientEnterNetworkHandler func(client *Client[T], data T) bool // Called after pipes adapter is registered, returns if the client should be disconnected (true = disconnect)

	// Determines the id of the event adapter for a client.
	ClientAdapterHandler func(client *Client[T]) string

	// Codec middleware
	ClientEncodingMiddleware func(client *Client[T], instance *Instance[T], message []byte) ([]byte, error)
	DecodingMiddleware       func(client *Client[T], instance *Instance[T], message []byte) ([]byte, error)

	// Error handler
	ErrorHandler func(err error)
}

// Message received from the client
type Message[T any] struct {
	Action string `json:"action"`
	Data   T      `json:"data"`
}

// Default pipes-fiber encoding middleware (using JSON)
func DefaultClientEncodingMiddleware[T any](client *Client[T], instance *Instance[T], message []byte) ([]byte, error) {
	return message, nil
}

// Default pipes-fiber decoding middleware (using JSON)
func DefaultDecodingMiddleware[T any](client *Client[T], instance *Instance[T], bytes []byte) ([]byte, error) {
	return bytes, nil
}

// Setup neogate using the config. Use the returned *Instance for interfacing with neogate.
func Setup[T any](config Config[T]) *Instance[T] {
	instance := &Instance[T]{
		Config:           config,
		adapters:         &sync.Map{},
		connectionsCache: &sync.Map{},
		sessionsCache:    &sync.Map{},
		routes:           make(map[string]func(*Context[T]) Event),
	}
	return instance
}

func (instance *Instance[T]) ReportGeneralError(context string, err error) {
	if instance.Config.ErrorHandler == nil {
		return
	}

	instance.Config.ErrorHandler(fmt.Errorf("general: %s: %s", context, err.Error()))
}

func (instance *Instance[T]) ReportClientError(client *Client[T], context string, err error) {
	if instance.Config.ErrorHandler == nil {
		return
	}

	instance.Config.ErrorHandler(fmt.Errorf("client %s: %s: %s", client.ID, context, err.Error()))
}
