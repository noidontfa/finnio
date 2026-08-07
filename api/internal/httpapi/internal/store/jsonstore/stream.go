package jsonstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"api/internal/httpapi/internal/entity"
	"api/internal/httpapi/internal/store"

	"github.com/google/uuid"
)

type StreamStore struct {
	mu       sync.RWMutex
	streams  []entity.Stream
	dataFile string
}

func New(dataDir string) (*StreamStore, error) {
	s := &StreamStore{
		dataFile: filepath.Join(dataDir, "streams.json"),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *StreamStore) load() error {
	b, err := os.ReadFile(s.dataFile)
	if err != nil {
		return fmt.Errorf("read streams mock data: %w", err)
	}

	var streams []entity.Stream
	if err := json.Unmarshal(b, &streams); err != nil {
		return fmt.Errorf("parse streams mock data: %w", err)
	}

	for _, stream := range streams {
		if !stream.Status.Valid() {
			return fmt.Errorf("parse streams mock data: stream %d has invalid status %q", stream.ID, stream.Status)
		}
	}

	s.mu.Lock()
	s.streams = streams
	s.mu.Unlock()
	return nil
}

func (s *StreamStore) saveLocked() error {
	b, err := json.MarshalIndent(s.streams, "", "  ")
	if err != nil {
		return fmt.Errorf("encode streams: %w", err)
	}
	b = append(b, '\n')

	if err := os.WriteFile(s.dataFile, b, 0o644); err != nil {
		return fmt.Errorf("write streams mock data: %w", err)
	}
	return nil
}

func (s *StreamStore) List(_ context.Context) ([]entity.Stream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]entity.Stream, len(s.streams))
	copy(out, s.streams)
	return out, nil
}

func (s *StreamStore) GetByID(_ context.Context, id int) (entity.Stream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, stream := range s.streams {
		if stream.ID == id {
			return stream, nil
		}
	}
	return entity.Stream{}, store.ErrNotFound
}

func (s *StreamStore) GetByKey(_ context.Context, key string) (entity.Stream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, stream := range s.streams {
		if stream.Key == key {
			return stream, nil
		}
	}
	return entity.Stream{}, store.ErrNotFound
}

func (s *StreamStore) UpdateStatusByKey(_ context.Context, key string, status entity.StreamStatus) (entity.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !status.Valid() {
		return entity.Stream{}, fmt.Errorf("invalid status %q", status)
	}

	for i, existing := range s.streams {
		if existing.Key != key {
			continue
		}

		updated := existing
		updated.Status = status

		prev := s.streams[i]
		s.streams[i] = updated
		if err := s.saveLocked(); err != nil {
			s.streams[i] = prev
			return entity.Stream{}, err
		}
		return updated, nil
	}

	return entity.Stream{}, store.ErrNotFound
}

func (s *StreamStore) Create(_ context.Context, stream entity.Stream) (entity.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stream.Name = strings.TrimSpace(stream.Name)
	if stream.Name == "" {
		return entity.Stream{}, fmt.Errorf("name is required")
	}
	if stream.Status == "" {
		stream.Status = entity.StatusIdle
	}
	if !stream.Status.Valid() {
		return entity.Stream{}, fmt.Errorf("invalid status %q", stream.Status)
	}

	key, err := newKey()
	if err != nil {
		return entity.Stream{}, err
	}
	stream.Key = key

	if stream.ID == 0 {
		stream.ID = s.nextIDLocked()
	}

	for _, existing := range s.streams {
		if existing.ID == stream.ID {
			return entity.Stream{}, store.ErrConflict
		}
	}

	s.streams = append(s.streams, stream)
	if err := s.saveLocked(); err != nil {
		s.streams = s.streams[:len(s.streams)-1]
		return entity.Stream{}, err
	}
	return stream, nil
}

func (s *StreamStore) Update(_ context.Context, id int, stream entity.Stream) (entity.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.streams {
		if existing.ID != id {
			continue
		}

		updated := existing
		if name := strings.TrimSpace(stream.Name); name != "" {
			updated.Name = name
		}
		if stream.Status != "" {
			if !stream.Status.Valid() {
				return entity.Stream{}, fmt.Errorf("invalid status %q", stream.Status)
			}
			updated.Status = stream.Status
		}

		prev := s.streams[i]
		s.streams[i] = updated
		if err := s.saveLocked(); err != nil {
			s.streams[i] = prev
			return entity.Stream{}, err
		}
		return updated, nil
	}

	return entity.Stream{}, store.ErrNotFound
}

func (s *StreamStore) Delete(_ context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, stream := range s.streams {
		if stream.ID != id {
			continue
		}

		prev := make([]entity.Stream, len(s.streams))
		copy(prev, s.streams)
		s.streams = append(s.streams[:i], s.streams[i+1:]...)
		if err := s.saveLocked(); err != nil {
			s.streams = prev
			return err
		}
		return nil
	}

	return store.ErrNotFound
}

func (s *StreamStore) nextIDLocked() int {
	maxID := 0
	for _, stream := range s.streams {
		if stream.ID > maxID {
			maxID = stream.ID
		}
	}
	return maxID + 1
}

func newKey() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return id.String(), nil
}
