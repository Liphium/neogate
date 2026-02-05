package neogate

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// Mount the neogate gateway using a fiber router.
func (instance *Instance[T]) MountGateway(router fiber.Router) {

	// Inject a middleware to check if the request is a websocket upgrade request
	router.Use("/", func(c *fiber.Ctx) error {

		// Check if it is a websocket upgrade request
		if websocket.IsWebSocketUpgrade(c) {
			info, ok := instance.Config.Handshake(c)
			if !ok {
				Log.Println("closed connection: invalid auth token")
				return c.SendStatus(fiber.StatusBadRequest)
			}

			// Create a unique session id to identify this specific session
			currentSession := GenerateToken(16)
			for instance.ExistsConnection(info.UserId, currentSession) {
				currentSession = GenerateToken(16)
			}
			info.sessionId = currentSession

			c.Locals("info", info)

			return c.Next()
		}

		return c.SendStatus(fiber.StatusUpgradeRequired)
	})

	// Mount an endpoint for actually receiving the websocket connection
	router.Get("/", websocket.New(func(c *websocket.Conn) {
		ws(c, instance)
	}))
}

// Handles the websocket connection
func ws[T any](conn *websocket.Conn, instance *Instance[T]) {
	deferFunc := func() {
		if err := recover(); err != nil {
			Log.Println("There was an error with a connection: ", err)
			debug.PrintStack()
		}

		// Close the connection
		conn.Close()
	}

	defer func() {
		deferFunc()
	}()

	// Get info from handshake in upgrade request
	info := conn.Locals("info").(SessionInfo[T])

	// Make sure there is an infinite read timeout again (1 week should be enough)
	conn.SetReadDeadline(time.Now().Add(time.Hour * 24 * 7))

	session := info.toSession(conn)
	instance.addSession(session)

	deferFunc = func() {

		// Recover from a failure (in case of a cast issue maybe?)
		if err := recover(); err != nil {
			Log.Println("connection with", info.UserId, "crashed cause of:", err)
		}

		// Get the session
		session, valid := instance.Get(info.UserId, info.sessionId)
		if !valid {
			return
		}

		// Remove the connection from the cache
		instance.Config.SessionDisconnectHandler(session)
		instance.RemoveSession(session.userId, session.sessionId)

		// Only remove adapter if all sessions are gone
		if len(instance.GetSessions(session.userId)) == 0 {
			instance.RemoveAdapter(session.userId)
		}
	}

	// Add adapter for pipes (if this is the first session)
	if len(instance.GetSessions(info.UserId)) == 1 {
		userAdapterName, _ := instance.Config.SessionAdapterHandler(session)
		instance.Adapt(CreateAction{
			ID: userAdapterName,
			OnEvent: func(c *AdapterContext) error {
				if err := instance.SendEventToUser(info.UserId, *c.Event); err != nil {
					instance.ReportSessionError(session, "couldn't send received message", err)
					return err
				}
				return nil
			},

			// Disconnect the user on error
			OnError: func(err error) {
				instance.RemoveAdapter(userAdapterName)
			},
		})
	}

	if instance.Config.SessionEnterNetworkHandler(session, info.Data) {
		return
	}

	for {
		_, msg, err := conn.ReadMessage()

		// Get the session
		session, valid := instance.Get(info.UserId, info.sessionId)
		if !valid {
			instance.ReportGeneralError("couldn't get session", fmt.Errorf("%s (%s)", info.UserId, info.sessionId))
			return
		}
		if err != nil {

			// Only log err if it is not due to expected connection closure
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				instance.ReportSessionError(session, "couldn't read message", err)
			}

			return
		}

		// Decode the message
		message, err := instance.Config.DecodingMiddleware(session, instance, msg)
		if err != nil {
			instance.ReportSessionError(session, "couldn't decode message", err)
			return
		}

		// Unmarshal the message to extract a few things
		var body map[string]any
		if err := sonic.Unmarshal(message, &body); err != nil {
			return
		}

		// Extract the response id and action from the message
		actionString, ok := body["action"].(string)
		if !ok {
			instance.ReportSessionError(session, "missing string field action", nil)
			return
		}
		args := strings.Split(actionString, ":")
		if len(args) != 2 {
			instance.ReportSessionError(session, "action field should consist of action:responseId", nil)
			return
		}
		action := args[0]
		responseId := args[1]

		ctx := &Context[T]{
			Session:    session,
			Data:       message,
			Action:     action,
			ResponseId: responseId,
			Instance:   instance,
		}

		// Handle the action
		if !instance.Handle(ctx) {
			instance.ReportSessionError(session, "couldn't handle action", fmt.Errorf("action=%s, response_id=%s", action, responseId))
			return
		}
	}
}
