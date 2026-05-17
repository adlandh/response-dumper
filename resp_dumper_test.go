package response

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"
)

func TestDumper(t *testing.T) {
	responseString := gofakeit.Sentence()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respDumper := NewDumper(w)
		w = respDumper
		w.WriteHeader(http.StatusOK)
		_, err := io.WriteString(w, responseString)
		require.NoError(t, err)
		require.Equal(t, respDumper.GetResponse(), responseString)
		require.Equal(t, http.StatusOK, respDumper.StatusCode())
		require.Equal(t, len(responseString), respDumper.BytesWritten())
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	require.Equal(t, w.Body.String(), responseString)
}

func TestDumperDefaultsStatusCode(t *testing.T) {
	responseString := gofakeit.Sentence()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respDumper := NewDumper(w)
		w = respDumper
		_, err := io.WriteString(w, responseString)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, respDumper.StatusCode())
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)
}

func TestDumperHijackNotSupported(t *testing.T) {
	w := httptest.NewRecorder()
	respDumper := NewDumper(w)

	conn, rw, err := respDumper.Hijack()
	require.ErrorIs(t, err, http.ErrNotSupported)
	require.Nil(t, conn)
	require.Nil(t, rw)
}

func TestDumperPushNotSupported(t *testing.T) {
	w := httptest.NewRecorder()
	respDumper := NewDumper(w)

	err := respDumper.Push("/resource", nil)
	require.ErrorIs(t, err, http.ErrNotSupported)
}

func TestDumperReadFrom(t *testing.T) {
	w := httptest.NewRecorder()
	respDumper := NewDumper(w)

	reader := strings.NewReader("hello world")
	n, err := respDumper.ReadFrom(reader)
	require.NoError(t, err)
	require.Equal(t, int64(len("hello world")), n)
	require.Equal(t, "hello world", respDumper.GetResponse())
	require.Equal(t, len("hello world"), respDumper.BytesWritten())
}

func TestDumperReadFromPartialError(t *testing.T) {
	w := httptest.NewRecorder()
	respDumper := NewDumper(w)

	reader := io.MultiReader(strings.NewReader("hello "), errReader{})
	n, err := respDumper.ReadFrom(reader)
	require.Error(t, err)
	require.Equal(t, int64(len("hello ")), n)
	require.Equal(t, "hello ", respDumper.GetResponse())
	require.Equal(t, len("hello "), respDumper.BytesWritten())
}

func TestDumperWriteHeaderOnce(t *testing.T) {
	w := httptest.NewRecorder()
	respDumper := NewDumper(w)

	respDumper.WriteHeader(http.StatusCreated)
	respDumper.WriteHeader(http.StatusAccepted)
	require.Equal(t, http.StatusCreated, respDumper.StatusCode())
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestDumperBody(t *testing.T) {
	w := httptest.NewRecorder()
	d := NewDumper(w)

	_, err := io.WriteString(d, "abc")
	require.NoError(t, err)
	require.Equal(t, []byte("abc"), d.Body())
}

func TestDumperUnwrap(t *testing.T) {
	w := httptest.NewRecorder()
	d := NewDumper(w)
	require.Same(t, w, d.Unwrap())
}

func TestDumperStatusCodeDefault(t *testing.T) {
	w := httptest.NewRecorder()
	d := NewDumper(w)
	require.Equal(t, http.StatusOK, d.StatusCode())
}

func TestDumperDefaultMaxBytesCap(t *testing.T) {
	w := httptest.NewRecorder()
	d := NewDumper(w)

	body := strings.Repeat("x", DefaultMaxBytes+500)
	n, err := io.WriteString(d, body)
	require.NoError(t, err)
	require.Equal(t, len(body), n)
	require.Equal(t, len(body), d.BytesWritten())
	require.Equal(t, DefaultMaxBytes, len(d.Body()))
	require.Equal(t, body, w.Body.String())
}

func TestDumperWithMaxBytesUnlimited(t *testing.T) {
	w := httptest.NewRecorder()
	d := NewDumper(w, WithMaxBytes(0))

	body := strings.Repeat("y", DefaultMaxBytes*3)
	_, err := io.WriteString(d, body)
	require.NoError(t, err)
	require.Equal(t, body, d.GetResponse())
}

func TestDumperWithMaxBytesCustom(t *testing.T) {
	w := httptest.NewRecorder()
	d := NewDumper(w, WithMaxBytes(5))

	_, err := io.WriteString(d, "hello world")
	require.NoError(t, err)
	require.Equal(t, "hello", d.GetResponse())
	require.Equal(t, "hello world", w.Body.String())
	require.Equal(t, len("hello world"), d.BytesWritten())

	_, err = io.WriteString(d, "!!!")
	require.NoError(t, err)
	require.Equal(t, "hello", d.GetResponse())
}

type flusherRecorder struct {
	*httptest.ResponseRecorder
	flushed int
}

func (f *flusherRecorder) Flush() { f.flushed++ }

func TestDumperFlush(t *testing.T) {
	f := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	d := NewDumper(f)
	d.Flush()
	d.Flush()
	require.Equal(t, 2, f.flushed)
}

type nonFlusherWriter struct{ http.ResponseWriter }

func TestDumperFlushNotSupported(t *testing.T) {
	d := NewDumper(nonFlusherWriter{httptest.NewRecorder()})
	require.NotPanics(t, func() { d.Flush() })
}

type errWriter struct {
	http.ResponseWriter
	err error
}

func (e errWriter) Write(_ []byte) (int, error) { return 0, e.err }

func TestDumperWriteError(t *testing.T) {
	want := errors.New("boom")
	d := NewDumper(errWriter{ResponseWriter: httptest.NewRecorder(), err: want})

	n, err := d.Write([]byte("hi"))
	require.ErrorIs(t, err, want)
	require.Zero(t, n)
	require.Zero(t, d.BytesWritten())
	require.Empty(t, d.Body())
}

type pusherRecorder struct {
	*httptest.ResponseRecorder
	target string
	err    error
}

func (p *pusherRecorder) Push(target string, _ *http.PushOptions) error {
	p.target = target
	return p.err
}

func TestDumperPushSuccess(t *testing.T) {
	p := &pusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	d := NewDumper(p)
	require.NoError(t, d.Push("/asset.css", nil))
	require.Equal(t, "/asset.css", p.target)
}

func TestDumperPushUnderlyingError(t *testing.T) {
	want := errors.New("push failed")
	p := &pusherRecorder{ResponseRecorder: httptest.NewRecorder(), err: want}
	d := NewDumper(p)
	require.ErrorIs(t, d.Push("/x", nil), want)
}

type hijackerRecorder struct {
	*httptest.ResponseRecorder
	conn net.Conn
	rw   *bufio.ReadWriter
	err  error
}

func (h *hijackerRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return h.conn, h.rw, h.err
}

func TestDumperHijackSuccess(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})
	rw := bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn))
	h := &hijackerRecorder{ResponseRecorder: httptest.NewRecorder(), conn: serverConn, rw: rw}
	d := NewDumper(h)

	conn, gotRW, err := d.Hijack()
	require.NoError(t, err)
	require.Same(t, serverConn, conn)
	require.Same(t, rw, gotRW)

	n, err := d.Write([]byte("after hijack"))
	require.ErrorIs(t, err, ErrHijacked)
	require.Zero(t, n)

	d.WriteHeader(http.StatusTeapot)
	require.Equal(t, http.StatusOK, d.StatusCode())
}

func TestDumperHijackUnderlyingError(t *testing.T) {
	want := errors.New("hijack failed")
	h := &hijackerRecorder{ResponseRecorder: httptest.NewRecorder(), err: want}
	d := NewDumper(h)

	_, _, err := d.Hijack()
	require.ErrorIs(t, err, want)

	_, err = d.Write([]byte("still here"))
	require.NoError(t, err)
}
