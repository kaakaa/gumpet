package chatter

// Seen remembers which headlines the pet has already said.
//
// The pet picks from the same few dozen headlines until the feeds change, so
// it repeats itself, and a headline said again looks exactly like a new one.
// Knowing which is which lets the balloon and the messages page mark the
// repeats, so the new ones stand out.
//
// A headline is known by its link: the same article under a reworded title is
// still the same article. Sayings out of a file are never marked; they have no
// link, and repeating is what they are for. It remembers for as long as gumpet
// runs, the same as the record of what was said.
//
// The zero value is ready.
type Seen struct {
	links map[string]struct{}
}

// Mark notes that r is being said, and reports whether it had been said
// before.
func (s *Seen) Mark(r Remark) (before bool) {
	if r.Link == "" {
		return false
	}
	if s.links == nil {
		s.links = map[string]struct{}{}
	}
	_, before = s.links[r.Link]
	s.links[r.Link] = struct{}{}
	return before
}
