package neogate

import (
	"fmt"
	"log"
	"sync"

	"github.com/gofiber/fiber/v2"
)

type None struct{}

var DebugLogs = true
var Log = log.New(log.Writer(), "neogate ", log.Flags())

type Instance[T any] struct {
	Config           Config[T]
	connectionsCache *sync.Map // UserId:sessionId -> *Session
	sessionsCache    SessionCache
	adapters         *sync.Map // AdapterId -> *Adapter
	routes           map[string]func(*Context[T]) Event
}

type SessionCache struct {
	mutex    *sync.Mutex
	sessions *sync.Map // UserId -> SessionsList
}

type SessionsList struct {
	mutex    *sync.RWMutex
	sessions []string // session ids
}

func (sessionList *SessionsList) Add(sessionId string) {
	sessionList.mutex.Lock()
	defer sessionList.mutex.Unlock()
	sessionList.sessions = append(sessionList.sessions, sessionId)
}

// ! If the functions aren't implemented pipesfiber will panic
// The generic should be the type of the handshake request
type Config[T any] struct {

	// Called when a client attempts to connect(create a session). Return the session info and true if the connection is allowed.
	// MUST BE SPECIFIED.
	Handshake func(c *fiber.Ctx) (SessionInfo[T], bool)

	// Session handlers
	SessionDisconnectHandler   func(session *Session[T])
	SessionEnterNetworkHandler func(session *Session[T], data T) bool // Called after pipes adapter is registered, returns if the client should be disconnected (true = disconnect)

	// Determines the id of the event adapter for a session.
	// Returns id of user adapter based on session.GetUserId(), and if of session adapter based on session.GetSessionId()
	SessionAdapterHandler func(userId string, sessionId string) (string, string)

	// Codec middleware
	EncodingMiddleware func(session *Session[T], instance *Instance[T], message []byte) ([]byte, error)
	DecodingMiddleware func(session *Session[T], instance *Instance[T], message []byte) ([]byte, error)

	// Error handler
	ErrorHandler func(err error)
}

// Message received from the session
type Message[T any] struct {
	Action string `json:"action"`
	Data   T      `json:"data"`
}

// Default pipes-fiber encoding middleware (using JSON)
func DefaultEncodingMiddleware[T any](session *Session[T], instance *Instance[T], message []byte) ([]byte, error) {
	return message, nil
}

// Default pipes-fiber decoding middleware (using JSON)
func DefaultDecodingMiddleware[T any](session *Session[T], instance *Instance[T], bytes []byte) ([]byte, error) {
	return bytes, nil
}

// Setup neogate using the config. Use the returned *Instance for interfacing with neogate.
func Setup[T any](config Config[T]) *Instance[T] {
	instance := &Instance[T]{
		Config:           config,
		adapters:         &sync.Map{},
		connectionsCache: &sync.Map{},
		sessionsCache: SessionCache{
			sessions: &sync.Map{},
			mutex:    &sync.Mutex{},
		},
		routes: make(map[string]func(*Context[T]) Event),
	}
	return instance
}

func (instance *Instance[T]) ReportGeneralError(context string, err error) {
	if instance.Config.ErrorHandler == nil {
		return
	}

	instance.Config.ErrorHandler(fmt.Errorf("general: %s: %s", context, err.Error()))
}

func (instance *Instance[T]) ReportSessionError(session *Session[T], context string, err error) {
	if instance.Config.ErrorHandler == nil {
		return
	}

	instance.Config.ErrorHandler(fmt.Errorf("session %s of user %s: %s: %s", session.sessionId, session.userId, context, err.Error()))
}
