package otopus

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v2/voice/udp"
	"github.com/diamondburned/vocalcord/player"
)

func TestIntegration(t *testing.T) {
	f, err := os.Open("testdata/1606807400801007782.dump.trimmed")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })

	u32buf := make([]byte, 4)
	sndbuf := make([]byte, 2048)

	packet := udp.Packet{
		Timestamp: 0,
	}

	pool, err := NewPool(1, time.Second)
	if err != nil {
		t.Fatal("failed to make pool:", err)
	}
	t.Cleanup(func() { pool.Close() })

	// Ensure that we could still play audio even when we're not using other
	// players.
	_ = testPoolGet(t, pool)
	_ = testPoolGet(t, pool)

	p := testPoolGet(t, pool)

	for {
		_, err := io.ReadFull(f, u32buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatal("failed to read header:", err)
		}

		u32 := binary.LittleEndian.Uint32(u32buf)

		_, err = io.ReadFull(f, sndbuf[:u32])
		if err != nil {
			t.Fatal("failed to read into sndbuf:", err)
		}

		packet.Sequence++
		packet.Timestamp += 960
		packet.Opus = sndbuf[:u32]

		if err := p.Write(&packet); err != nil {
			t.Fatal("failed to write packet:", err)
		}
	}
}

func testPoolGet(t *testing.T, pool *Pool) player.PlayerSession {
	p, err := pool.Get()
	if err != nil {
		t.Fatal("failed to get from pool:", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}
