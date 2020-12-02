package pool

import (
	"sync"
	"time"

	"github.com/diamondburned/vocalcord/player"
	"github.com/pkg/errors"
)

type PlayerSessionCreator = func() (player.PlayerSession, error)

type Pool struct {
	maxAge  time.Duration
	maxSize int

	create PlayerSessionCreator

	mutex     sync.Mutex
	sessions  map[player.PlayerSession]time.Time
	creating  int
	createErr error

	ticker  time.Ticker
	stopper chan struct{}
}

func NewPool(maxSize int, maxAge time.Duration, fn PlayerSessionCreator) *Pool {
	p := &Pool{
		maxAge:   maxAge,
		maxSize:  maxSize,
		create:   fn,
		sessions: map[player.PlayerSession]time.Time{},
		// Operate a cleanup ticker at half the resolution of maxAge.
		ticker:  *time.NewTicker(maxAge / 2),
		stopper: make(chan struct{}),
	}

	p.startGC()

	return p
}

func (p *Pool) startGC() {
	go func() {
		for {
			select {
			case <-p.stopper:
				return
			case now := <-p.ticker.C:
				p.cleanup(now)
			}
		}
	}()
}

func (p *Pool) cleanup(now time.Time) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for session, time := range p.sessions {
		if time.Add(p.maxAge).Before(now) {
			delete(p.sessions, session)
		}
	}
}

// Stop stops the internal worker for Pool and all stored sessions.
func (p *Pool) Stop() {
	p.ticker.Stop()
	close(p.stopper)

	// A busy cleanup might still have the lock.
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for session := range p.sessions {
		session.Close()
		delete(p.sessions, session)
	}
}

// Fill fills up the pool with sessions up to min(n, p.maxSize).
func (p *Pool) Fill(n int) error {
	if n > p.maxSize {
		n = p.maxSize
	}

	var sessions = make([]player.PlayerSession, 0, n)
	for n > 0 {
		s, err := p.create()
		if err != nil {
			return err
		}
		sessions = append(sessions, s)
		n--
	}

	now := time.Now()

	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, session := range sessions {
		if !p.tryPut(session, now) {
			session.Close()
		}
	}

	return nil
}

// Get tries to get a session from the pool. It creates a new session if there
// is none.
func (p *Pool) Get() (player.PlayerSession, error) {
	s, err := p.tryGet(false)
	if err == nil {
		return s, nil
	}

	return p.create()
}

var ErrNoSessions = errors.New("no sessions in pool")

// TryGet tries to get a session. If there is none in the pool, it will queue a
// request to make one.
func (p *Pool) TryGet() (player.PlayerSession, error) {
	s, err := p.tryGet(true)
	if err == nil {
		return s, nil
	}

	return nil, ErrNoSessions
}

func (p *Pool) tryGet(createIfDrained bool) (player.PlayerSession, error) {
	// Always return nil if we're stopped.
	select {
	case <-p.stopper:
		return nil, ErrNoSessions
	default:
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.createErr != nil {
		return nil, p.createErr
	}

	for session := range p.sessions {
		delete(p.sessions, session)
		return session, nil
	}

	if p.creating < p.maxSize {
		p.creating++

		go func() {
			s, err := p.create()
			if err == nil {
				p.Put(s) // wasteful
				return
			}

			p.mutex.Lock()
			p.creating = 0
			p.createErr = err
			p.mutex.Unlock()
		}()
	}

	return nil, ErrNoSessions
}

// Put puts a session back into the pool. The caller should always use Put OR
// Close, but not both. If Pool is filled, then this method will close the
// Session internally. As such, it should be used over Close.
func (p *Pool) Put(s player.PlayerSession) {
	if !p.tryPut(s, time.Now()) {
		s.Close()
	}
}

func (p *Pool) tryPut(s player.PlayerSession, now time.Time) bool {
	select {
	case <-p.stopper:
		return false
	default:
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.creating > 0 {
		p.creating--
	}

	if len(p.sessions) < p.maxSize {
		p.sessions[s] = now
		return true
	}
	return false
}
