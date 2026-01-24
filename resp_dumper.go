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

type Dumper struct {
	http.ResponseWriter

	mw           io.Writer
	buf          *bytes.Buffer
	statusCode   int
	wroteHeader  bool
	bytesWritten int
}

func NewDumper(respWriter http.ResponseWriter) *Dumper {
	buf := new(bytes.Buffer)

	return &Dumper{
		ResponseWriter: respWriter,

		mw:  io.MultiWriter(respWriter, buf),
		buf: buf,
	}
}

func (d *Dumper) Write(b []byte) (int, error) {
	if !d.wroteHeader {
		d.WriteHeader(http.StatusOK)
	}

	nBytes, err := d.mw.Write(b)
	if err != nil {
		err = fmt.Errorf("error writing response: %w", err)
	}

	d.bytesWritten += nBytes

	return nBytes, err
}

func (d *Dumper) GetResponse() string {
	return d.buf.String()
}

func (d *Dumper) StatusCode() int {
	if d.wroteHeader {
		return d.statusCode
	}

	return http.StatusOK
}

func (d *Dumper) BytesWritten() int {
	return d.bytesWritten
}

func (d *Dumper) WriteHeader(statusCode int) {
	if d.wroteHeader {
		return
	}

	d.statusCode = statusCode
	d.wroteHeader = true
	d.ResponseWriter.WriteHeader(statusCode)
}

func (d *Dumper) Flush() {
	if flusher, ok := d.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (d *Dumper) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := d.ResponseWriter.(http.Pusher); ok {
		if err := pusher.Push(target, opts); err != nil {
			return fmt.Errorf("error pushing resource: %w", err)
		}

		return nil
	}

	return http.ErrNotSupported
}

func (d *Dumper) ReadFrom(r io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)

	var total int64

	for {
		n, err := r.Read(buf)
		if n > 0 {
			written, writeErr := d.Write(buf[:n])
			total += int64(written)

			if writeErr != nil {
				return total, writeErr
			}

			if written < n {
				return total, io.ErrShortWrite
			}
		}

		if err != nil {
			if err == io.EOF {
				return total, nil
			}

			return total, fmt.Errorf("error reading response source: %w", err)
		}
	}
}

func (d *Dumper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := d.ResponseWriter.(http.Hijacker); ok {
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			err = fmt.Errorf("error hijacking response: %w", err)
		}

		return conn, rw, err
	}

	return nil, nil, http.ErrNotSupported
}

func (d *Dumper) Unwrap() http.ResponseWriter {
	return d.ResponseWriter
}
