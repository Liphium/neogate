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

	defer func() {
		if err := recover(); err != nil {
			Log.Println("There was an error with a connection: ", err)
			debug.PrintStack()
		}

		// Close the connection
		conn.Close()
	}()

	// Let the connection time out after 30 seconds
	conn.SetReadDeadline(time.Now().Add(time.Second * instance.Config.HandshakeTimeout))

	// Complete the handshake
	var handshakePacket T
	if !isTypeNone(handshakePacket) {

		// Get handshake packet
		if err := conn.ReadJSON(&handshakePacket); err != nil {
			Log.Println("closed connection: couldn't decode auth packet: ", err)
			return
		}
	}
	info, ok := instance.Config.Handshake(handshakePacket)
	if !ok {
		Log.Println("closed connection: invalid auth token")
		return
	}

	// Make sure the session isn't already connected
	if instance.ExistsConnection(info.ID, info.Session) {
		Log.Println("closed connection: already connected")
		return
	}

	// Make sure there is an infinite read timeout again (1 week should be enough)
	conn.SetReadDeadline(time.Now().Add(time.Hour * 24 * 7))

	client := instance.AddClient(info.ToClient(conn))
	defer func() {

		// Recover from a failure (in case of a cast issue maybe?)
		if err := recover(); err != nil {
			Log.Println("connection with", client.ID, "crashed cause of:", err)
		}

		// Get the client
		client, valid := instance.Get(info.ID, info.Session)
		if !valid {
			return
		}

		// Remove the connection from the cache
		instance.Config.ClientDisconnectHandler(client)
		instance.Remove(info.ID, info.Session)

		// Only remove adapter if all sessions are gone
		if len(instance.GetSessions(info.ID)) == 0 {
			instance.RemoveAdapter(info.ID)
		}
	}()

	// Add adapter for pipes (if this is the first session)
	if len(instance.GetSessions(info.ID)) == 1 {
		adapterName := instance.Config.ClientAdapterHandler(client)
		instance.Adapt(CreateAction{
			ID: adapterName,
			OnEvent: func(c *AdapterContext) error {
				if err := instance.SendToAccount(info.ID, c.Message); err != nil {
					instance.ReportClientError(client, "couldn't send received message", err)
					return err
				}
				return nil
			},

			// Disconnect the user on error
			OnError: func(err error) {
				instance.RemoveAdapter(adapterName)
			},
		})
	}

	if instance.Config.ClientEnterNetworkHandler(client, handshakePacket) {
		return
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {

			// Get the client for error reporting purposes
			client, valid := instance.Get(info.ID, info.Session)
			if !valid {
				instance.ReportGeneralError("couldn't get client", fmt.Errorf("%s (%s)", info.ID, info.Session))
				return
			}

			instance.ReportClientError(client, "couldn't read message", err)
			return
		}

		// Get the client
		client, valid := instance.Get(info.ID, info.Session)
		if !valid {
			instance.ReportGeneralError("couldn't get client", fmt.Errorf("%s (%s)", info.ID, info.Session))
			return
		}

		// Decode the message
		message, err := instance.Config.DecodingMiddleware(client, instance, msg)
		if err != nil {
			instance.ReportClientError(client, "couldn't decode message", err)
			return
		}

		// Unmarshal the message to extract a few things
		var body map[string]interface{}
		if err := sonic.Unmarshal(message, &body); err != nil {
			return
		}

		// Extract the response id from the message
		args := strings.Split(body["action"].(string), ":")
		if len(args) != 2 {
			return
		}

		ctx := &Context[T]{
			Client:     client,
			Data:       message,
			Action:     args[0],
			ResponseId: args[1],
			Instance:   instance,
		}

		// Handle the action
		if !instance.Handle(ctx) {
			instance.ReportClientError(client, "couldn't handle action", fmt.Errorf("action=%s, response_id=%s", args[0], args[1]))
			return
		}
	}
}
