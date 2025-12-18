package neogate

import (
	"runtime/debug"
)

type NormalResponseStruct struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func NormalResponse[T any](ctx *Context[T], data interface{}) Event {
	return Response(ctx, data, ctx.Instance)
}

func SuccessResponse[T any](ctx *Context[T]) Event {
	return Response(ctx, NormalResponseStruct{
		Success: true,
	}, ctx.Instance)
}

func ErrorResponse[T any](ctx *Context[T], message string, err error) Event {

	if DebugLogs {
		Log.Println("error with action "+ctx.Action+" (", message, "): ", err)
		debug.PrintStack()
	}

	return Response(ctx, NormalResponseStruct{
		Success: false,
		Message: message,
	}, ctx.Instance)
}

func Response[T any](ctx *Context[T], data interface{}, instance *Instance[T]) Event {
	return Event{
		Name: "res:" + ctx.Action + ":" + ctx.ResponseId,
		Data: data,
	}
}
