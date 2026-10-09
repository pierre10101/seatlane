package domain

import "testing"

// Exactly one state is true for every combination, for a signed-in viewer
// and for one who is not signed in (0), and expiry is exclusive: a hold whose
// expires_at equals now has expired.
func TestExactlyOneSeatState(t *testing.T) {
	const me, other, now = 1, 2, 1000
	for _, viewer := range []int64{me, 0} {
		for _, heldBy := range []int64{0, me, other} {
			for _, expires := range []int64{0, now - 1, now, now + 1} {
				for _, soldTo := range []int64{0, me, other} {
					got := []bool{
						IsAvailable(heldBy, expires, soldTo, now),
						IsHeldBy(heldBy, expires, soldTo, viewer, now),
						IsHeldByOther(heldBy, expires, soldTo, viewer, now),
						IsSoldTo(soldTo, viewer),
						IsSoldToOther(soldTo, viewer),
					}
					n := 0
					for _, b := range got {
						if b {
							n++
						}
					}
					if n != 1 {
						t.Errorf("viewer %d held_by %d expires %d sold_to %d: %v", viewer, heldBy, expires, soldTo, got)
					}
				}
			}
		}
	}
	if IsHeldBy(me, now, 0, me, now) || !IsAvailable(me, now, 0, now) {
		t.Fatal("a hold expiring exactly now has expired")
	}
	if IsHeldBy(0, now+1, 0, 0, now) || IsSoldTo(0, 0) {
		t.Fatal("a viewer who is not signed in holds and owns nothing")
	}
}
