# response-dumper

Tiny Go helper that wraps an `http.ResponseWriter` and captures the response body.

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

	"github.com/adlandh/response-dumper"
)

func handler(w http.ResponseWriter, r *http.Request) {
	d := response.NewDumper(w)
	w = d

	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "hello")

	// Inspect response
	_ = d.GetResponse()
	_ = d.StatusCode()
	_ = d.BytesWritten()
}
```

## Notes

- Implements `http.Flusher`, `http.Hijacker`, `http.Pusher`, and `io.ReaderFrom` when supported by the wrapped writer.
- `Hijack`/`Push` return `http.ErrNotSupported` when unavailable.

## Testing

```bash
go test ./...
```
