package domain

// SeatView is one seat on the seat map, with its state for the viewing
// user. Exactly one of the five state flags is true. It has no hold
// expiry: the seat map never tells anyone when a hold runs out; a visitor's
// own hold times come from MyHold (list_my_holds).
//
// bridge-en: a seat on the map
type SeatView struct {
	SeatID      int64  `json:"seat_id"`
	Section     string `json:"section"`
	SectionRank int64  `json:"section_rank"`
	Row         string `json:"row"`
	Number      int64  `json:"number"`
	Price       Money  `json:"price"`
	Available   bool   `json:"available"`
	HeldByMe    bool   `json:"held_by_me"`
	HeldByOther bool   `json:"held_by_other"`
	SoldToMe    bool   `json:"sold_to_me"`
	SoldToOther bool   `json:"sold_to_other"`
}

// MyHold is one seat the signed-in user holds, with when its hold ends.
// Active is false once expires_at is no later than now (the seat is then
// free for anyone, until this user holds it again).
//
// bridge-en: a hold of mine
type MyHold struct {
	SeatID    int64 `json:"seat_id"`
	HeldAt    int64 `json:"held_at"`
	ExpiresAt int64 `json:"expires_at"`
	Active    bool  `json:"active"`
}

// IsActiveHold: a hold is active while it ends later than now.
func IsActiveHold(expiresAt, now int64) bool {
	return expiresAt > now
}

// IsAvailable: not sold, and nobody holds it or the hold has expired.
func IsAvailable(heldBy, expiresAt, soldTo, now int64) bool {
	return soldTo == 0 && (heldBy == 0 || expiresAt <= now)
}

// IsHeldBy: not sold, held by the viewer (a signed-in user, from 1 up), and
// the hold has not expired. A viewer who is not signed in (0) holds nothing.
func IsHeldBy(heldBy, expiresAt, soldTo, viewer, now int64) bool {
	return soldTo == 0 && heldBy != 0 && heldBy == viewer && expiresAt > now
}

// IsHeldByOther: not sold, held by someone other than the viewer, and the
// hold has not expired.
func IsHeldByOther(heldBy, expiresAt, soldTo, viewer, now int64) bool {
	return soldTo == 0 && heldBy != 0 && heldBy != viewer && expiresAt > now
}

// IsSoldTo: sold to the viewer (a viewer who is not signed in, 0, bought nothing).
func IsSoldTo(soldTo, viewer int64) bool {
	return soldTo != 0 && soldTo == viewer
}

// IsSoldToOther: sold to someone other than the viewer.
func IsSoldToOther(soldTo, viewer int64) bool {
	return soldTo != 0 && soldTo != viewer
}
