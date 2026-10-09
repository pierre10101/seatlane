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
	for _, bad := range []string{"", "0", "-3", "+3", "abc", "9007199254740992"} {
		if _, ok := Parse(bad); ok {
			t.Fatalf("%q parsed", bad)
		}
	}
}

// Issue mints a cookie only when the request has no valid one, and never
// changes the request (no body or query injection).
func TestIssueSetsCookieAndLeavesRequestAlone(t *testing.T) {
	var gotBody, gotQuery string
	h := Issue(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotQuery = string(b), r.URL.RawQuery
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/holds?limit=5", strings.NewReader(`{"seat_id":3}`)))
	if gotBody != `{"seat_id":3}` || gotQuery != "limit=5" {
		t.Fatalf("request changed: body %s query %s", gotBody, gotQuery)
	}
	c := rec.Result().Cookies()
	if len(c) != 1 || c[0].Name != Cookie || Cookie != "bridge_session" || !c[0].HttpOnly || c[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie %v", c)
	}
	if _, ok := Parse(c[0].Value); !ok {
		t.Fatalf("minted %q", c[0].Value)
	}
	for value, mints := range map[string]bool{"42": false, "0": true, "abc": true} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: Cookie, Value: value})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := len(rec.Result().Cookies()) == 1; got != mints {
			t.Fatalf("cookie %q: minted=%v", value, got)
		}
	}
}
