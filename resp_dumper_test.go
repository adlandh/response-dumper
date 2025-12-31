package response

import (
	"io"
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
