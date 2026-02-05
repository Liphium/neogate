package neogate

import (
	"errors"
	"time"
)

// RemoveSession a session from the users sessions (DOES NOT DISCONNECT, there is an extra method for that)
func (instance *Instance[T]) RemoveSession(userId string, sessionId string) {

	// Disconnect in case there was a panic
	instance.DisconnectSession(userId, sessionId)

	// Cleanup session
	instance.connectionsCache.Delete(getKey(userId, sessionId))
	instance.removeSession(userId, sessionId)
}

// DisconnectSession closes/disconnects the session
func (instance *Instance[T]) DisconnectSession(userId string, sessionId string) {

	// Get the session
	session, valid := instance.Get(userId, sessionId)
	if !valid {
		instance.ReportGeneralError("session "+sessionId+" of user "+userId+" doesn't exist", errors.New("couldn't delete"))
		return
	}

	session.wsMutex.Lock()
	defer session.wsMutex.Unlock()

	// This is a little weird for disconnecting, but it works, so I'm not complaining
	session.conn.SetReadDeadline(time.Now().Add(time.Microsecond * 1))
	if err := session.conn.Close(); err != nil {
		instance.ReportGeneralError("couldn't disconnect session", err)
	}
}

func (instance *Instance[T]) ExistsConnection(userId string, sessionId string) bool {
	_, ok := instance.connectionsCache.Load(getKey(userId, sessionId))
	if !ok {
		return false
	}

	return ok
}

func (instance *Instance[T]) Get(userId string, sessionId string) (*Session[T], bool) {
	session, valid := instance.connectionsCache.Load(getKey(userId, sessionId))
	if !valid {
		return nil, false
	}

	return session.(*Session[T]), true
}

func (instance *Instance[T]) GetConnections(userId string) int {
	sessions, ok := instance.sessionsCache.Load(userId)
	if !ok {
		return 0
	}

	return len(sessions.([]string))
}
