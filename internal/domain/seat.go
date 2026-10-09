package domain

// SeatView is one seat on the seat map, with its state for the viewing
// session. Exactly one of the five state flags is true.
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
	ExpiresAt   int64  `json:"expires_at"`
}

// IsAvailable: not sold, and nobody holds it or the hold has expired.
func IsAvailable(heldBy, expiresAt, soldTo, now int64) bool {
	return soldTo == 0 && (heldBy == 0 || expiresAt <= now)
}

// IsHeldBy: not sold, held by session, and the hold has not expired.
// Sessions are positive (F8), so held_by = session means somebody holds it.
func IsHeldBy(heldBy, expiresAt, soldTo, session, now int64) bool {
	return soldTo == 0 && heldBy == session && expiresAt > now
}

// IsHeldByOther: not sold, held by another session, and the hold has not expired.
func IsHeldByOther(heldBy, expiresAt, soldTo, session, now int64) bool {
	return soldTo == 0 && heldBy != 0 && heldBy != session && expiresAt > now
}

// IsSoldTo: sold to session (sessions are positive, so it is sold).
func IsSoldTo(soldTo, session int64) bool {
	return soldTo == session
}

// IsSoldToOther: sold to another session.
func IsSoldToOther(soldTo, session int64) bool {
	return soldTo != 0 && soldTo != session
}
