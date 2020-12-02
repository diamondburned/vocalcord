package file

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Session struct {
	f *os.File
	b []byte
}

func NewSession(dir string) (*Session, error) {
	now := time.Now().UnixNano()

	f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%d.dump", now)))
	if err != nil {
		return nil, err
	}

	return &Session{f, make([]byte, 4)}, nil
}

func (s *Session) Write(b []byte) (int, error) {
	binary.LittleEndian.PutUint32(s.b, uint32(len(b)))

	n, err := s.f.Write(s.b)
	if err != nil {
		return n, err
	}

	return s.f.Write(b)
}

func (s *Session) Close() error {
	return s.f.Close()
}
