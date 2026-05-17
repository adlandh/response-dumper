// Package response implements a response writer that dumps the response body
package response

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
)

// Dumper wraps an http.ResponseWriter and mirrors every byte written through
// it into an in-memory buffer, so the response body can be inspected after the
// handler returns (e.g. for logging or auditing middleware).
//
// A Dumper is not safe for concurrent use. All methods must be called from the
// same goroutine as the handler, or after the handler has returned.
//
// By default the buffer is capped at DefaultMaxBytes. Use WithMaxBytes to
// override; once the cap is reached, further bytes still flow to the network
// but are not appended to the buffer. Pass a non-positive value to disable
// the cap entirely.
type Dumper struct {
	http.ResponseWriter

	buf          *bytes.Buffer
	maxBytes     int
	statusCode   int
	bytesWritten int
	wroteHeader  bool
	hijacked     bool
}

// ErrHijacked is returned by Write or WriteHeader when called after a
// successful Hijack.
var ErrHijacked = fmt.Errorf("response: connection has been hijacked")

// DefaultMaxBytes is the default in-memory buffer cap used by NewDumper when
// no WithMaxBytes option is supplied.
const DefaultMaxBytes = 1000

// Option configures a Dumper at construction time.
type Option func(*Dumper)

// WithMaxBytes caps the in-memory buffer at n bytes. Writes beyond the cap are
// still forwarded to the underlying ResponseWriter but are not recorded in the
// buffer returned by GetResponse. A non-positive n disables the cap (unlimited
// buffering). If this option is not supplied, the cap is DefaultMaxBytes.
func WithMaxBytes(n int) Option {
	return func(d *Dumper) {
		d.maxBytes = n
	}
}

// NewDumper returns a Dumper that wraps respWriter. Optional Options may be
// passed to customize behavior; see WithMaxBytes.
func NewDumper(respWriter http.ResponseWriter, opts ...Option) *Dumper {
	d := &Dumper{
		ResponseWriter: respWriter,
		buf:            new(bytes.Buffer),
		maxBytes:       DefaultMaxBytes,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Write forwards b to the underlying ResponseWriter and appends it to the
// in-memory buffer (subject to any WithMaxBytes cap). If WriteHeader has not
// been called yet, it is called with http.StatusOK first, matching the
// behavior of http.ResponseWriter.
func (d *Dumper) Write(b []byte) (int, error) {
	if d.hijacked {
		return 0, ErrHijacked
	}

	if !d.wroteHeader {
		d.WriteHeader(http.StatusOK)
	}

	n, err := d.ResponseWriter.Write(b)
	if n > 0 {
		d.appendToBuf(b[:n])
		d.bytesWritten += n
	}

	if err != nil {
		return n, fmt.Errorf("error writing response: %w", err)
	}

	return n, nil
}

func (d *Dumper) appendToBuf(b []byte) {
	if d.maxBytes <= 0 {
		d.buf.Write(b)
		return
	}

	remaining := d.maxBytes - d.buf.Len()
	if remaining <= 0 {
		return
	}

	if len(b) > remaining {
		b = b[:remaining]
	}

	d.buf.Write(b)
}

// GetResponse returns the buffered response body as a string. The returned
// value is a copy of the buffer's contents at call time. Prefer Body to avoid
// the copy when a []byte view is acceptable.
func (d *Dumper) GetResponse() string {
	return d.buf.String()
}

// Body returns the buffered response body as a byte slice aliasing the
// internal buffer. The slice is only valid until the next Write; callers must
// not modify it. Use GetResponse for a stable copy.
func (d *Dumper) Body() []byte {
	return d.buf.Bytes()
}

// StatusCode returns the status code passed to WriteHeader, or
// http.StatusOK if WriteHeader has not been called.
func (d *Dumper) StatusCode() int {
	if d.wroteHeader {
		return d.statusCode
	}

	return http.StatusOK
}

// BytesWritten returns the total number of body bytes successfully written to
// the underlying ResponseWriter.
func (d *Dumper) BytesWritten() int {
	return d.bytesWritten
}

// WriteHeader records statusCode and forwards it to the underlying
// ResponseWriter. Subsequent calls are no-ops, matching net/http semantics.
func (d *Dumper) WriteHeader(statusCode int) {
	if d.hijacked || d.wroteHeader {
		return
	}

	d.statusCode = statusCode
	d.wroteHeader = true
	d.ResponseWriter.WriteHeader(statusCode)
}

// Flush invokes Flush on the underlying ResponseWriter if it implements
// http.Flusher; otherwise it is a no-op.
func (d *Dumper) Flush() {
	if flusher, ok := d.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Push implements http.Pusher by delegating to the underlying ResponseWriter.
// It returns http.ErrNotSupported if the underlying writer does not implement
// http.Pusher.
func (d *Dumper) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := d.ResponseWriter.(http.Pusher); ok {
		if err := pusher.Push(target, opts); err != nil {
			return fmt.Errorf("error pushing resource: %w", err)
		}

		return nil
	}

	return http.ErrNotSupported
}

// writerOnly hides any ReaderFrom implementation on the wrapped Writer so that
// io.Copy falls back to repeated Write calls.
type writerOnly struct{ io.Writer }

// ReadFrom implements io.ReaderFrom so that io.Copy(dumper, src) routes
// through the Dumper's Write method (which both forwards to the underlying
// ResponseWriter and records to the buffer). It does not enable any zero-copy
// path, because the Dumper must observe every byte.
func (d *Dumper) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.Copy(writerOnly{Writer: d}, r)
	if err != nil {
		return n, fmt.Errorf("error reading response source: %w", err)
	}

	return n, nil
}

// Hijack implements http.Hijacker by delegating to the underlying
// ResponseWriter. It returns http.ErrNotSupported if the underlying writer
// does not implement http.Hijacker. After a successful Hijack the Dumper must
// not be written to.
func (d *Dumper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := d.ResponseWriter.(http.Hijacker); ok {
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			return conn, rw, fmt.Errorf("error hijacking response: %w", err)
		}

		d.hijacked = true

		return conn, rw, nil
	}

	return nil, nil, http.ErrNotSupported
}

// Unwrap returns the underlying ResponseWriter, enabling http.ResponseController
// to find capabilities (Flusher, Hijacker, ...) on the wrapped writer.
func (d *Dumper) Unwrap() http.ResponseWriter {
	return d.ResponseWriter
}
