package neogate

import (
	"errors"

	"github.com/bytedance/sonic"
	"github.com/gofiber/websocket/v2"
)

// SendEventToUser sends the event to all sessions connected to the userId
func (instance *Instance[T]) SendEventToUser(userId string, event Event) error {
	sessionIds, ok := instance.sessionsCache.Load(userId)
	if !ok {
		return errors.New("no sessions found")
	}

	if err := instance.Send(sessionIds.([]string), event); err != nil {
		return err
	}
	return nil
}

// Sends an event to a specific Session
func (instance *Instance[T]) SendEventToSession(c *Session[T], event Event) error {
	msg, err := sonic.Marshal(event)
	if err != nil {
		return err
	}

	err = instance.sendToSessionWS(c, msg)
	return err
}

func (instance *Instance[T]) sendToSessionWS(session *Session[T], msg []byte) error {

	msg, err := instance.Config.EncodingMiddleware(session, instance, msg)
	if err != nil {
		return err
	}

	// Lock and unlock mutex after writing
	session.wsMutex.Lock()
	defer session.wsMutex.Unlock()

	return session.conn.WriteMessage(websocket.BinaryMessage, msg)
}

// Send an event to all adapters
func (instance *Instance[T]) Send(adapters []string, event Event) error {
	msg, err := sonic.Marshal(event)
	if err != nil {
		return err
	}

	for _, adapter := range adapters {
		instance.AdapterReceive(adapter, event, msg)
	}
	return nil
}

// Sends an event to the account.
//
// Only returns errors for encoding, not retrieval (cause adapters handle that themselves).
func (instance *Instance[T]) SendOne(adapter string, event Event) error {
	return instance.Send([]string{adapter}, event)
}
