package session

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestMintAndParse(t *testing.T) {
	for i := 0; i < 1000; i++ {
		id := Mint()
		if _, ok := Parse(strconv.FormatInt(id, 10)); !ok {
			t.Fatalf("minted %d does not parse", id)
		}
	}
	for _, bad := range []string{"", "0", "-3", "abc", "9007199254740992"} {
		if _, ok := Parse(bad); ok {
			t.Fatalf("%q parsed", bad)
		}
	}
}

func TestWrapInjectsSession(t *testing.T) {
	var gotBody, gotQuery string
	h := Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotQuery = string(b), r.URL.RawQuery
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/holds", strings.NewReader(`{"seat_id":3}`))
	req.AddCookie(&http.Cookie{Name: Cookie, Value: "42"})
	h.ServeHTTP(httptest.NewRecorder(), req)
	if gotBody != `{"seat_id":3,"session":42}` {
		t.Fatalf("body %s", gotBody)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/events/1/seats?limit=5", nil)
	req.AddCookie(&http.Cookie{Name: Cookie, Value: "42"})
	h.ServeHTTP(httptest.NewRecorder(), req)
	if gotQuery != "limit=5&session=42" {
		t.Fatalf("query %s", gotQuery)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"session":1}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("caller-sent session: %d", rec.Code)
	}
}
