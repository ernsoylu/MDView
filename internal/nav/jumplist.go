package nav

// Pos is one jumplist position. Offset is exact for the layout it was
// recorded in; SourceLine is the resize-proof fallback.
type Pos struct {
	Path       string
	Offset     int
	SourceLine int
}

const maxJumps = 100

// Jumplist is the unified back/forward history: link follows, TOC jumps,
// and search jumps push onto it.
type Jumplist struct {
	back, fwd []Pos
}

// Push records cur as a place to come back to and clears forward history.
func (j *Jumplist) Push(cur Pos) {
	j.back = append(j.back, cur)
	if len(j.back) > maxJumps {
		j.back = j.back[len(j.back)-maxJumps:]
	}
	j.fwd = nil
}

// Back pops the previous position, remembering cur for Forward.
func (j *Jumplist) Back(cur Pos) (Pos, bool) {
	if len(j.back) == 0 {
		return Pos{}, false
	}
	p := j.back[len(j.back)-1]
	j.back = j.back[:len(j.back)-1]
	j.fwd = append(j.fwd, cur)
	return p, true
}

// Forward pops the next position, remembering cur for Back.
func (j *Jumplist) Forward(cur Pos) (Pos, bool) {
	if len(j.fwd) == 0 {
		return Pos{}, false
	}
	p := j.fwd[len(j.fwd)-1]
	j.fwd = j.fwd[:len(j.fwd)-1]
	j.back = append(j.back, cur)
	return p, true
}
