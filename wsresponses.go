package neogate

import (
	"runtime/debug"
)

type NormalResponseStruct struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func NormalResponse(ctx *Context, data interface{}) Event {
	return Response(ctx, data, ctx.Instance)
}

func SuccessResponse(ctx *Context) Event {
	return Response(ctx, NormalResponseStruct{
		Success: true,
	}, ctx.Instance)
}

func ErrorResponse(ctx *Context, message string, err error) Event {

	if DebugLogs {
		Log.Println("error with action "+ctx.Action+" (", message, "): ", err)
		debug.PrintStack()
	}

	return Response(ctx, NormalResponseStruct{
		Success: false,
		Message: message,
	}, ctx.Instance)
}

func Response(ctx *Context, data interface{}, instance *Instance) Event {
	return Event{
		Name: "res:" + ctx.Action + ":" + ctx.ResponseId,
		Data: data,
	}
}
