package otopus

import (
	"github.com/diamondburned/arikawa/v2/voice/udp"
	"github.com/pkg/errors"
	"layeh.com/gopus"
)

const (
	sampleRate = 48000
	channels   = 2
)

type opusDecoder struct {
	*gopus.Decoder
}

func newOpusdec() (*opusDecoder, error) {
	dec, err := gopus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, err
	}
	return &opusDecoder{dec}, nil
}

type opusStateful struct {
	decoder *opusDecoder
	lastTs  uint32
	seqnce  uint16
}

func wrapStateful(dec *opusDecoder) opusStateful {
	return opusStateful{decoder: dec}
}

var errDecoderClosed = errors.New("decoder closed")

// Read reads the UDP Opus packet and returns PCM.
func (s *opusStateful) Read(p *udp.Packet) ([]int16, error) {
	if s.decoder == nil {
		return nil, errDecoderClosed
	}
	if s.seqnce >= p.Sequence {
		return nil, nil
	}
	fs := p.Timestamp - s.lastTs
	s.lastTs = p.Timestamp
	s.seqnce = p.Sequence
	return s.decoder.Decode(p.Opus, int(fs), false)
}

func (s *opusStateful) Close() error {
	s.decoder = nil
	return nil
}
