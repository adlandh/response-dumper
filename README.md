# response-dumper

Tiny Go helper that wraps an `http.ResponseWriter` and captures the response body
for inspection after the handler runs (logging, auditing, debugging).

## Install

```bash
go get github.com/adlandh/response-dumper
```

## Usage

```go
package main

import (
	"io"
	"net/http"

	response "github.com/adlandh/response-dumper"
)

func handler(w http.ResponseWriter, r *http.Request) {
	d := response.NewDumper(w)
	w = d

	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "hello")

	// Inspect response after writing
	_ = d.GetResponse()  // string copy of buffered body
	_ = d.Body()         // []byte view (do not mutate, invalid after next Write)
	_ = d.StatusCode()
	_ = d.BytesWritten()
}
```

## Buffer cap

The in-memory buffer is capped at `DefaultMaxBytes` (1000) by default. Bytes
beyond the cap still flow through to the network but are not recorded in the
buffer. Override with `WithMaxBytes`:

```go
// Cap buffer at 64 KiB
d := response.NewDumper(w, response.WithMaxBytes(64*1024))

// Disable cap (unlimited buffering — beware of large responses)
d := response.NewDumper(w, response.WithMaxBytes(0))
```

## Notes

- Implements `http.Flusher`, `http.Hijacker`, `http.Pusher`, and `io.ReaderFrom`
  when supported by the wrapped writer.
- `Hijack` / `Push` return `http.ErrNotSupported` when unavailable.
- After a successful `Hijack`, further `Write` calls return `ErrHijacked` and
  `WriteHeader` is a no-op.
- A `Dumper` is not safe for concurrent use; call its methods from the handler
  goroutine, or only after the handler has returned.

## Testing

```bash
go test ./...
```
