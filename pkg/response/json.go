package response

import (
	"bytes"
	"sync"

	json "github.com/goccy/go-json"
	"github.com/valyala/fasthttp"
)

// bufPool reuses byte buffers for JSON encoding — avoids allocation per response.
var bufPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 512))
	},
}

// JSON sends a JSON response with status code.
// Uses goccy/go-json (3-5× faster) and pooled buffers (zero-alloc hot path).
func JSON(ctx *fasthttp.RequestCtx, statusCode int, data interface{}) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	ctx.Response.Header.SetContentType("application/json")
	ctx.SetStatusCode(statusCode)

	if err := json.NewEncoder(buf).Encode(data); err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		_, _ = ctx.Write([]byte(`{"error":"internal server error"}`))
		return
	}

	_, _ = ctx.Write(buf.Bytes())
}

// Error sends a JSON error response with status code
func Error(ctx *fasthttp.RequestCtx, statusCode int, errMsg string) {
	JSON(ctx, statusCode, map[string]string{"error": errMsg})
}
