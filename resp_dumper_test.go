package response

import (
	"io"
	"net/http"
	"net/http/httptest"
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
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	require.Equal(t, w.Body.String(), responseString)
}
