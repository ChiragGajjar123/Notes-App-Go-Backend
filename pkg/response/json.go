package response

import (
	"encoding/json"

	"github.com/valyala/fasthttp"
)

// JSON sends a JSON response with status code
func JSON(ctx *fasthttp.RequestCtx, statusCode int, data interface{}) {
	ctx.Response.Header.SetContentType("application/json")
	ctx.SetStatusCode(statusCode)
	_ = json.NewEncoder(ctx).Encode(data)
}

// Error sends a JSON error response with status code
func Error(ctx *fasthttp.RequestCtx, statusCode int, errMsg string) {
	JSON(ctx, statusCode, map[string]string{"error": errMsg})
}

