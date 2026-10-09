package domain

// SeatView is one seat on the seat map, with its state for the viewing
// session. Exactly one of the five state flags is true. It has no hold
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

// MyHold is one seat the viewing session holds, with when its hold ends.
// Active is false once expires_at is no later than now (the seat is then
// free for anyone, until this session holds it again).
//
// bridge-en: a session hold
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
func IsAvailable(heldBy string, expiresAt int64, soldTo string, now int64) bool {
	return soldTo == "" && (heldBy == "" || expiresAt <= now)
}

// IsHeldBy: not sold, held by session, and the hold has not expired.
// Sessions are never empty (F8), so held_by = session means somebody holds it.
func IsHeldBy(heldBy string, expiresAt int64, soldTo, session string, now int64) bool {
	return soldTo == "" && heldBy == session && expiresAt > now
}

// IsHeldByOther: not sold, held by another session, and the hold has not expired.
func IsHeldByOther(heldBy string, expiresAt int64, soldTo, session string, now int64) bool {
	return soldTo == "" && heldBy != "" && heldBy != session && expiresAt > now
}

// IsSoldTo: sold to session (sessions are never empty, so it is sold).
func IsSoldTo(soldTo, session string) bool {
	return soldTo == session
}

// IsSoldToOther: sold to another session.
func IsSoldToOther(soldTo, session string) bool {
	return soldTo != "" && soldTo != session
}
