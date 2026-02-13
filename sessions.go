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
	_, loaded := instance.connectionsCache.LoadOrStore(getKey(session.userId, session.sessionId), session)

	// If the session is not yet added, make sure to add it to the list
	if !loaded {
		instance.createSession(session)
	}
}

func getKey(id string, session string) string {
	return id + ":" + session
}

func (instance *Instance[T]) createSession(session *Session[T]) {

	_, sessionAdapterName := instance.Config.SessionAdapterHandler(session.GetUserId(), session.GetSessionId())
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

	sessionList := &SessionsList{
		mutex:    &sync.RWMutex{},
		sessions: []string{session.GetSessionId()},
	}
	sessionAny, loaded := instance.sessions.LoadOrStore(session.GetUserId(), sessionList)
	if loaded {
		sessionList = sessionAny.(*SessionsList)
	}

	sessionList.mutex.Lock()
	defer sessionList.mutex.Unlock()

	sessionList.sessions = append(sessionList.sessions, session.GetSessionId())
	instance.sessions.Store(session.GetUserId(), sessionList) // Store here in case the thing was deleted while the mutex was locked
}

func (instance *Instance[T]) removeSession(userId string, session string) {

	// Remove adapter for session
	_, sessionAdapter := instance.Config.SessionAdapterHandler(userId, session)
	instance.RemoveAdapter(sessionAdapter)

	sessionAny, ok := instance.sessions.Load(userId)
	if !ok {
		return
	}
	sessionList := sessionAny.(*SessionsList)

	sessionList.mutex.Lock()
	defer sessionList.mutex.Unlock()

	sessionList.sessions = slices.DeleteFunc(sessionList.sessions, func(s string) bool {
		return s == session
	})

	if len(sessionList.sessions) == 0 {
		instance.sessions.Delete(userId)
	}
}

func (instance *Instance[T]) GetSessions(userId string) []string {
	sessionListAny, ok := instance.sessions.Load(userId)
	if !ok {
		return []string{}
	}
	sessionList := sessionListAny.(*SessionsList)
	sessionList.mutex.RLock()
	sessionIds := make([]string, len(sessionList.sessions))
	copy(sessionIds, sessionList.sessions)
	sessionList.mutex.RUnlock()

	return sessionIds
}
