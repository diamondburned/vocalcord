package otopus

import (
	"time"

	"github.com/diamondburned/vocalcord/player"
	"github.com/diamondburned/vocalcord/player/pool"
	"github.com/hajimehoshi/oto"
	"github.com/pkg/errors"
)

type Pool struct {
	*pool.Pool
	ctx *oto.Context
}

func NewPool(maxSize int, maxAge time.Duration) (*Pool, error) {
	dec, err := newOpusdec()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create opus decoder")
	}

	ctx, err := newContext()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create oto context")
	}

	p := pool.NewPool(maxSize, maxAge, func() (player.PlayerSession, error) {
		return newPlayer(dec, ctx), nil
	})

	return &Pool{p, ctx}, nil
}

func (p *Pool) Close() error {
	return p.ctx.Close()
}
