package neogate

import (
	"slices"
	"sync"

	"github.com/gofiber/websocket/v2"
)

// Used to provide information for session creation
type SessionInfo[T any] struct {
	UserId    string // Identifier of the user
	sessionId string // Identifier of this user session
	Data      T      // Session data you can decide how to fill
}

// Convert the session information to a session that can be used by neogate.
func (sessionInfo SessionInfo[T]) toSession(conn *websocket.Conn) *Session[T] {

	return &Session[T]{
		conn:      conn,
		userId:    sessionInfo.UserId,
		sessionId: sessionInfo.sessionId,
		data:      sessionInfo.Data,
		wsMutex:   &sync.Mutex{},
		dataMutex: &sync.RWMutex{},
	}
}

type Session[T any] struct {
	conn      *websocket.Conn
	userId    string
	sessionId string
	data      T
	dataMutex *sync.RWMutex
	wsMutex   *sync.Mutex
}

func (session *Session[T]) GetData() T {
	session.dataMutex.RLock()
	defer session.dataMutex.RUnlock()

	return session.data
}

func (session *Session[T]) SetData(data T) {
	session.dataMutex.Lock()
	defer session.dataMutex.Unlock()

	session.data = data
}

func (session *Session[T]) GetUserId() string {
	return session.userId
}

func (session *Session[T]) GetSessionId() string {
	return session.sessionId
}

func (instance *Instance[T]) addSession(session *Session[T]) {

	// Add the session
	_, valid := instance.connectionsCache.Load(getKey(session.userId, session.sessionId))
	instance.connectionsCache.Store(getKey(session.userId, session.sessionId), session)

	// If the session is not yet added, make sure to add it to the list
	if !valid {
		instance.createSession(session)
	}
}

func getKey(id string, session string) string {
	return id + ":" + session
}

func (instance *Instance[T]) createSession(session *Session[T]) {
	userId := session.userId
	sessionID := session.sessionId

	_, sessionAdapterName := instance.Config.SessionAdapterHandler(session)
	instance.Adapt(CreateAction{
		ID: sessionAdapterName,
		OnEvent: func(c *AdapterContext) error {
			if err := instance.sendToSessionWS(session, c.Message); err != nil {
				instance.ReportSessionError(session, "couldn't send received message", err)
				return err
			}
			return nil
		},

		// Disconnect the user on error
		OnError: func(err error) {
			instance.RemoveAdapter(sessionAdapterName)
		},
	})

	sessions, valid := instance.sessionsCache.Load(userId)
	if valid {
		instance.sessionsCache.Store(userId, append(sessions.([]string), sessionID))
	} else {
		instance.sessionsCache.Store(userId, []string{sessionID})
	}
}

func (instance *Instance[T]) removeSession(userId string, session string) {

	// Remove adapter for session
	instance.RemoveAdapter(session)

	sessions, valid := instance.sessionsCache.Load(userId)
	if valid {

		if len(sessions.([]string)) == 1 {
			instance.sessionsCache.Delete(userId)
			return
		}

		instance.sessionsCache.Store(userId, slices.DeleteFunc(sessions.([]string), func(s string) bool {
			return s == session
		}))
	}
}

func (instance *Instance[T]) GetSessions(id string) []string {
	sessions, valid := instance.sessionsCache.Load(id)
	if valid {
		return sessions.([]string)
	}

	return []string{}
}
