// Package devclock lets a developer move the server clock forward, to watch
// holds expire without waiting 10 minutes. It replaces httpx.Now, the only
// clock read on the request path, so every action still gets the time
// passed in (T1). Off unless the server runs with -dev-clock.
package devclock

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/pierre10101/go-ai-bridge/runtime/httpx"
)

var offset atomic.Int64 // seconds

// Install makes httpx.Now the wall clock plus the current offset.
func Install() {
	httpx.Now = func() time.Time { return time.Now().Add(time.Duration(offset.Load()) * time.Second) }
}

// Handler answers POST /__dev/advance?seconds=N with the new offset.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		n, err := strconv.ParseInt(r.URL.Query().Get("seconds"), 10, 64)
		if err != nil || n < 0 || n > 86400 {
			http.Error(w, "seconds must be 0..86400", http.StatusBadRequest)
			return
		}
		total := offset.Add(n)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{"offset_seconds": total, "now": httpx.Now().Unix()})
	})
}
