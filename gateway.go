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
			for instance.ExistsConnection(info.ID, currentSession) {
				currentSession = GenerateToken(16)
			}
			info.sessionID = currentSession

			// Make sure the session isn't already connected
			if instance.ExistsConnection(info.ID, info.sessionID) {
				Log.Println("closed connection: already connected")
				return c.SendStatus(fiber.StatusBadRequest)
			}

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
	info := conn.Locals("info").(ClientInfo[T])

	// Make sure there is an infinite read timeout again (1 week should be enough)
	conn.SetReadDeadline(time.Now().Add(time.Hour * 24 * 7))

	client := info.ToClient(conn)
	instance.AddClient(client)

	deferFunc = func() {

		// Recover from a failure (in case of a cast issue maybe?)
		if err := recover(); err != nil {
			Log.Println("connection with", info.ID, "crashed cause of:", err)
		}

		// Get the client
		client, valid := instance.Get(info.ID, info.sessionID)
		if !valid {
			return
		}

		// Remove the connection from the cache
		instance.Config.ClientDisconnectHandler(client)
		instance.Remove(client.ID, client.Session)

		// Only remove adapter if all sessions are gone
		if len(instance.GetSessions(client.ID)) == 0 {
			instance.RemoveAdapter(client.ID)
		}
	}

	// Add adapter for pipes (if this is the first session)
	if len(instance.GetSessions(info.ID)) == 1 {
		accountAdapterName, _ := instance.Config.ClientAdapterHandler(client)
		instance.Adapt(CreateAction{
			ID: accountAdapterName,
			OnEvent: func(c *AdapterContext) error {
				if err := instance.SendToAccount(info.ID, *c.Event); err != nil {
					instance.ReportClientError(client, "couldn't send received message", err)
					return err
				}
				return nil
			},

			// Disconnect the user on error
			OnError: func(err error) {
				instance.RemoveAdapter(accountAdapterName)
			},
		})
	}

	if instance.Config.ClientEnterNetworkHandler(client, info.Data) {
		return
	}

	for {
		_, msg, err := conn.ReadMessage()

		// Get the client
		client, valid := instance.Get(info.ID, info.sessionID)
		if !valid {
			instance.ReportGeneralError("couldn't get client", fmt.Errorf("%s (%s)", info.ID, info.sessionID))
			return
		}
		if err != nil {

			// Only log err if it is not due to expected connection closure
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				instance.ReportClientError(client, "couldn't read message", err)
			}

			return
		}

		// Decode the message
		message, err := instance.Config.DecodingMiddleware(client, instance, msg)
		if err != nil {
			instance.ReportClientError(client, "couldn't decode message", err)
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
			instance.ReportClientError(client, "missing string field action", err)
			return
		}
		args := strings.Split(actionString, ":")
		if len(args) != 2 {
			return
		}
		action := args[0]
		responseId := args[1]

		ctx := &Context[T]{
			Client:     client,
			Data:       message,
			Action:     action,
			ResponseId: responseId,
			Instance:   instance,
		}

		// Handle the action
		if !instance.Handle(ctx) {
			instance.ReportClientError(client, "couldn't handle action", fmt.Errorf("action=%s, response_id=%s", action, responseId))
			return
		}
	}
}
