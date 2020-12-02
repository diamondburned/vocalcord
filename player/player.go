package player

import (
	"io"
	"log"
	"sync"

	"github.com/diamondburned/arikawa/v2/discord"
	"github.com/diamondburned/arikawa/v2/voice"
	"github.com/diamondburned/arikawa/v2/voice/udp"
	"github.com/diamondburned/arikawa/v2/voice/voicegateway"
	"github.com/pkg/errors"
)

// ErrSSRCNotFound is returned when the main UDP read loop stumbles on a packet
// with an unknown SSRC.
var ErrSSRCNotFound = errors.New("ssrc from UDP not found")

type PlayerSession interface {
	Write(*udp.Packet) error
	io.Closer
}

type PlayerSessionPool interface {
	Put(PlayerSession)
	Get() (PlayerSession, error)
	TryGet() (PlayerSession, error)
}

// Player contains a structure to map an SSRC to an active mpv session.
type Player struct {
	pool PlayerSessionPool

	vcses   *voice.Session
	cancels []func()

	ssrcMu   sync.Mutex
	ssrcs    map[discord.UserID]uint32
	sessions map[uint32]PlayerSession
	sesErr   error
}

func NewPlayer(vcses *voice.Session, pool PlayerSessionPool) *Player {
	pl := &Player{
		pool:     pool,
		vcses:    vcses,
		ssrcs:    map[discord.UserID]uint32{},
		sessions: map[uint32]PlayerSession{},
	}
	pl.cancels = []func(){
		vcses.AddHandler(pl.onSpeaking),
		vcses.AddHandler(pl.onClientConnect),
		vcses.AddHandler(pl.onClientDisconnect),
	}
	return pl
}

func (pl *Player) onSpeaking(speaking *voicegateway.SpeakingEvent) {
	pl.Register(speaking.UserID, speaking.SSRC)
	log.Println("User is speaking =", speaking.Speaking != voicegateway.NotSpeaking)
}

func (pl *Player) onClientConnect(connect *voicegateway.ClientConnectEvent) {
	pl.Register(connect.UserID, connect.AudioSSRC)
	log.Println("Client connected:", connect.UserID)
}

func (pl *Player) onClientDisconnect(disconn *voicegateway.ClientDisconnectEvent) {
	var session PlayerSession

	pl.ssrcMu.Lock()

	ssrc, ok := pl.ssrcs[disconn.UserID]
	if ok {
		session = pl.sessions[ssrc]

		delete(pl.ssrcs, disconn.UserID)
		delete(pl.sessions, ssrc)
	}

	pl.ssrcMu.Unlock()

	if session != nil {
		pl.pool.Put(session)
	}
}

// Register registers the user ID to the SSRC and makes a new mpv session
// internally for that SSRC.
func (pl *Player) Register(userID discord.UserID, ssrc uint32) {
	pl.ssrcMu.Lock()
	// Check if we already have this user registered.
	_, ok := pl.ssrcs[userID]
	// Check if we have a fatal error that we cannot continue.
	err := pl.sesErr
	pl.ssrcMu.Unlock()

	if ok || err != nil {
		return
	}

	s, err := pl.pool.Get()
	if err != nil {
		pl.ssrcMu.Lock()
		pl.sesErr = err
		pl.ssrcMu.Unlock()

		return
	}

	pl.ssrcMu.Lock()

	if pl.sesErr == nil {
		pl.ssrcs[userID] = ssrc
		pl.sessions[ssrc] = s
	}

	pl.ssrcMu.Unlock()
}

func (pl *Player) playerSession(ssrc uint32) (PlayerSession, error) {
	pl.ssrcMu.Lock()
	defer pl.ssrcMu.Unlock()

	if pl.sesErr != nil {
		return nil, pl.sesErr
	}

	s, ok := pl.sessions[ssrc]
	if ok {
		return s, nil
	}

	return nil, ErrSSRCNotFound
}

// Start blocks until either an error is received or conn is closed.
func (pl *Player) Start() error {
	defer pl.freeSessions()

	uconn := pl.vcses.VoiceUDPConn()

	for {
		p, err := uconn.ReadPacket()
		if err != nil {
			return errors.Wrap(err, "failed to read packet")
		}

		s, err := pl.playerSession(p.SSRC)
		if err != nil {
			if err == ErrSSRCNotFound {
				log.Println("Dropping SSRC", p.SSRC)
				continue
			}

			return errors.Wrap(err, "failed to get opus session")
		}

		if err = s.Write(p); err != nil {
			log.Println("Failed to write to mpv pipe:", err)
			continue
		}
	}
}

func (pl *Player) freeSessions() {
	pl.ssrcMu.Lock()
	defer pl.ssrcMu.Unlock()

	// Set an error so Register calls won't set to the map.
	pl.sesErr = errors.New("player is closed")

	for _, session := range pl.sessions {
		pl.pool.Put(session)
	}

	pl.ssrcs = nil
	pl.sessions = nil

	for _, cancel := range pl.cancels {
		cancel()
	}
}
