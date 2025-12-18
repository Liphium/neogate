package neogate

import (
	"fmt"
	"log"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type None struct{}

var DebugLogs = true
var Log = log.New(log.Writer(), "neogate ", log.Flags())

type Instance struct {
	Config           Config
	connectionsCache *sync.Map // ID:Session -> Client
	sessionsCache    *sync.Map // ID -> Session list
	adapters         *sync.Map // ID -> Adapter
	routes           map[string]func(*Context) Event
}

type ClientInfo struct {
	ID      string // Identifier of the account
	Session string // Identifier of the session
	Data    any    // Session data you can decide how to fill
}

// Convert the client information to a client that can be used by neogate.
func (info ClientInfo) ToClient(conn *websocket.Conn) Client {
	return Client{
		Conn:    conn,
		ID:      info.ID,
		Session: info.Session,
		Data:    info.Data,
		Mutex:   &sync.Mutex{},
	}
}

// ! If the functions aren't implemented pipesfiber will panic
// The generic should be the type of the handshake request
type Config struct {

	// Called when a client attempts to connection. Return true if the connection is allowed.
	// MUST BE SPECIFIED.
	Handshake func(c *fiber.Ctx) (ClientInfo, bool)

	// Client handlers
	ClientDisconnectHandler func(client *Client)

	// Called after pipes adapter is registered, returns if the client should be disconnected (true = disconnect)
	// data is of the type of data you saved in ClientInfo.Data
	ClientEnterNetworkHandler func(client *Client, data any) bool

	// Determines the id of the event adapter for a client.
	ClientAdapterHandler func(client *Client) string

	// Codec middleware
	ClientEncodingMiddleware func(client *Client, instance *Instance, message []byte) ([]byte, error)
	DecodingMiddleware       func(client *Client, instance *Instance, message []byte) ([]byte, error)

	// Error handler
	ErrorHandler func(err error)
}

// Message received from the client
type Message[T any] struct {
	Action string `json:"action"`
	Data   T      `json:"data"`
}

// Default pipes-fiber encoding middleware (using JSON)
func DefaultClientEncodingMiddleware(client *Client, instance *Instance, message []byte) ([]byte, error) {
	return message, nil
}

// Default pipes-fiber decoding middleware (using JSON)
func DefaultDecodingMiddleware(client *Client, instance *Instance, bytes []byte) ([]byte, error) {
	return bytes, nil
}

// Setup neogate using the config. Use the returned *Instance for interfacing with neogate.
func Setup(config Config) *Instance {
	instance := &Instance{
		Config:           config,
		adapters:         &sync.Map{},
		connectionsCache: &sync.Map{},
		sessionsCache:    &sync.Map{},
		routes:           make(map[string]func(*Context) Event),
	}
	return instance
}

func (instance *Instance) ReportGeneralError(context string, err error) {
	if instance.Config.ErrorHandler == nil {
		return
	}

	instance.Config.ErrorHandler(fmt.Errorf("general: %s: %s", context, err.Error()))
}

func (instance *Instance) ReportClientError(client *Client, context string, err error) {
	if instance.Config.ErrorHandler == nil {
		return
	}

	instance.Config.ErrorHandler(fmt.Errorf("client %s: %s: %s", client.ID, context, err.Error()))
}
