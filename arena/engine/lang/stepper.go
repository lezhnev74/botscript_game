package lang

// The stepper runs an Interp on its own goroutine in strict lockstep with the
// goroutine that calls Decide: exactly one of the two runs at any time, and
// every handover is a channel operation, so the schedule (and GOMAXPROCS)
// cannot change what the script sees or does.

type entryKind int

// Entry points, loop first so the zero entry is "start loop()".
const (
	entLoop entryKind = iota
	entHit
	entBump
)

type entry struct {
	kind entryKind
	arg  int
}

func (e entry) args() []int {
	if e.kind == entLoop {
		return nil
	}
	return []int{e.arg}
}

func (p *Program) entryFunc(k entryKind) *Func {
	switch k {
	case entHit:
		return p.OnHit
	case entBump:
		return p.OnBump
	}
	return p.Loop
}

// directive is what the driver tells the blocked script on a new beat.
type directive struct {
	enter bool  // abort the call stack and start to
	to    entry // the entry point to start
	quit  bool  // stop the goroutine
}

// quitSignal unwinds the goroutine on Close. It is not Unwind, so Interp.Run
// lets it through.
type quitSignal struct{}

type stepper struct {
	in    *Interp
	wake  chan directive
	out   chan Action // the action of the beat, or IDLE; closed when the goroutine ends
	done  chan struct{}
	fault any

	// Written by the script goroutine only while the driver is blocked, and
	// read by the driver only while the script is blocked.
	mode  entryKind // the entry point on the stack
	entry entry     // where to restart after an unwind
	acted bool      // the running loop() has queued an action
	fell  int       // loop() returns without an action that were made a wait

	started, dead bool
}

func newStepper(in *Interp) *stepper {
	return &stepper{in: in, wake: make(chan directive), out: make(chan Action), done: make(chan struct{})}
}

// step hands the beat to the script and waits for its action. ok is false
// when the goroutine has ended (Close, or a bug in the interpreter).
func (s *stepper) step(d directive) (a Action, ok bool) {
	if s.dead {
		return Action{}, false
	}
	if s.started {
		s.wake <- d
	} else {
		s.started = true
		go s.main(d.to)
	}
	a, ok = <-s.out
	s.dead = !ok
	return a, ok
}

// close stops the goroutine and waits until it has gone.
func (s *stepper) close() {
	if s.started && !s.dead {
		s.wake <- directive{quit: true}
		<-s.out
	}
	if s.started {
		<-s.done
	}
	s.dead = true
}

func (s *stepper) main(e entry) {
	defer close(s.done)
	defer s.finish()
	for {
		e = s.runOnce(e)
	}
}

// finish records an interpreter bug (anything but a quit) and releases the
// driver.
func (s *stepper) finish() {
	if r := recover(); r != nil {
		if _, quit := r.(quitSignal); !quit {
			s.fault = r
		}
	}
	close(s.out)
}

// runOnce runs one entry point to its end and returns the next one.
func (s *stepper) runOnce(e entry) entry {
	s.mode, s.acted = e.kind, false
	if s.in.Run(s.in.Prog.entryFunc(e.kind), e.args()...) {
		return s.entry
	}
	return s.afterReturn(e)
}

// afterReturn: a loop() that ended without an action makes this beat a wait
// (no free spinning); then loop() starts fresh.
func (s *stepper) afterReturn(e entry) entry {
	if e.kind == entLoop && !s.acted {
		s.fell++
		if next, ok := s.yield(Action{Kind: ActWait}); ok {
			return next
		}
	}
	return entry{}
}

// yield gives the beat's action to the driver and blocks until the next beat.
func (s *stepper) yield(a Action) (next entry, enter bool) {
	s.out <- a
	d := <-s.wake
	if d.quit {
		panic(quitSignal{})
	}
	return d.to, d.enter
}

func (s *stepper) block(a Action) {
	if e, ok := s.yield(a); ok {
		s.entry = e
		panic(Unwind{})
	}
}

// Do implements Actions.
func (s *stepper) Do(a Action) {
	s.acted = true
	s.block(a)
}

// OutOfBudget implements Actions: the beat is missed (IDLE).
func (s *stepper) OutOfBudget() { s.block(Action{Kind: ActIdle}) }
