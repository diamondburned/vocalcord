package otopus

import (
	"encoding/binary"
	"sync"

	"github.com/diamondburned/arikawa/v2/voice/udp"
	"github.com/hajimehoshi/oto"
	"github.com/pkg/errors"
)

var (
	otoContext  *oto.Context
	otoError    error
	onceContext sync.Once
)

func newContext() (*oto.Context, error) {
	onceContext.Do(func() {
		// Internally, oto seems to use bitdepth=2 for 16-bit signed.
		otoContext, otoError = oto.NewContext(48000, 2, 2, 4096)
	})

	return otoContext, otoError
}

type otoPlayer struct {
	pcm  chan []int16
	err  chan error
	opus opusStateful
}

var empty = make([]byte, 32)

func newPlayer(dec *opusDecoder, ctx *oto.Context) *otoPlayer {
	ch := make(chan []int16)
	er := make(chan error, 1)

	// TODO: fork oto to remove this loop. We could fix this on oto's side by
	// exposing its mux.AddSource API instead of relying on the pipe.

	go func() {
		pl := ctx.NewPlayer()
		defer pl.Close()

		for {
			select {
			case p, ok := <-ch:
				if !ok {
					return
				}

				if err := binary.Write(pl, binary.LittleEndian, p); err != nil {
					er <- errors.Wrap(err, "binary error")
					return
				}

			default:
				_, err := pl.Write(empty)
				if err != nil {
					er <- errors.Wrap(err, "write empty error")
					return
				}
			}
		}
	}()

	return &otoPlayer{
		pcm:  ch,
		err:  er,
		opus: wrapStateful(dec),
	}
}

func (player *otoPlayer) Write(p *udp.Packet) error {
	pcm, err := player.opus.Read(p)
	if err != nil {
		return errors.Wrap(err, "failed to read opus packet")
	}

	select {
	case player.pcm <- pcm:
		return nil
	case err := <-player.err:
		return err
	}
}

func (player *otoPlayer) Close() error {
	close(player.pcm)
	player.opus.Close()
	return nil
}
