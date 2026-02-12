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

	sessionAny, ok := instance.sessionsCache.sessions.Load(session.GetUserId())
	if ok {
		sessionList := sessionAny.(*SessionsList)

		sessionList.mutex.Lock()
		defer sessionList.mutex.Unlock()

		if sessionAny, ok = instance.sessionsCache.sessions.Load(session.GetUserId()); ok {
			sessionList = sessionAny.(*SessionsList)
			sessionList.sessions = append(sessionList.sessions, session.GetSessionId())
			instance.sessionsCache.sessions.Store(session.GetUserId(), sessionList)
			return
		}
	}

	instance.sessionsCache.mutex.Lock()

	sessionAny, ok = instance.sessionsCache.sessions.Load(session.GetUserId())
	if ok {
		sessionList := sessionAny.(*SessionsList)
		instance.sessionsCache.mutex.Unlock()

		sessionList.Add(session.GetSessionId())
	} else {
		instance.sessionsCache.sessions.Store(session.GetUserId(), &SessionsList{
			mutex:    &sync.RWMutex{},
			sessions: []string{session.GetSessionId()},
		})
		instance.sessionsCache.mutex.Unlock()
	}
}

func (instance *Instance[T]) removeSession(userId string, session string) {

	// Remove adapter for session
	_, sessionAdapter := instance.Config.SessionAdapterHandler(userId, session)
	instance.RemoveAdapter(sessionAdapter)

	instance.sessionsCache.mutex.Lock()
	defer instance.sessionsCache.mutex.Unlock()

	sessionAny, ok := instance.sessionsCache.sessions.Load(userId)
	if !ok {
		return
	}
	sessionList := sessionAny.(*SessionsList)

	sessionList.mutex.Lock()
	defer sessionList.mutex.Unlock()

	sessionList.sessions = slices.DeleteFunc(sessionList.sessions, func(s string) bool {
		return s == session
	})
	instance.sessionsCache.sessions.Store(userId, sessionList)

	if len(sessionList.sessions) == 0 {
		instance.sessionsCache.sessions.Delete(userId)
	}
}

func (instance *Instance[T]) GetSessions(userId string) []string {
	sessionList, ok := instance.sessionsCache.sessions.Load(userId)
	if !ok {
		return []string{}
	}
	sessions := sessionList.(*SessionsList)
	sessions.mutex.RLock()
	sessionIds := sessions.sessions
	sessions.mutex.RUnlock()

	return sessionIds
}
