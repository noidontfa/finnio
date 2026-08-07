package entity

import (
	"fmt"
	"strings"

	"api/gen/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// StreamStatus represents the lifecycle state of a stream.
// swagger:enum StreamStatus
type StreamStatus string

const (
	StatusIdle   StreamStatus = "idle"
	StatusReady  StreamStatus = "ready"
	StatusLive   StreamStatus = "live"
	StatusPaused StreamStatus = "paused"
	StatusEnded  StreamStatus = "ended"
	StatusFailed StreamStatus = "failed"
)

// Stream is a financial or data stream resource.
type Stream struct {
	ID     int          `json:"id" example:"1"`
	Key    string       `json:"key" example:"a3f2c891-4b5e-4d7a-9c12-8f4e6d2a1b90"`
	Name   string       `json:"name" example:"Salary"`
	Status StreamStatus `json:"status" enums:"idle,ready,live,paused,ended,failed" example:"ready"`
}

func (s StreamStatus) Valid() bool {
	switch s {
	case StatusIdle, StatusReady, StatusLive, StatusPaused, StatusEnded, StatusFailed:
		return true
	default:
		return false
	}
}

// CreateStreamRequest is the payload for creating a stream.
type CreateStreamRequest struct {
	Name   string       `json:"name" validate:"required" example:"Salary"`
	Status StreamStatus `json:"status" validate:"omitempty" enums:"idle,ready,live,paused,ended,failed" example:"idle"`
}

// UpdateStreamRequest is the payload for updating a stream.
type UpdateStreamRequest struct {
	Name   string       `json:"name" example:"Salary"`
	Status StreamStatus `json:"status" enums:"idle,ready,live,paused,ended,failed" example:"paused"`
}

// StreamResponse includes stream data and its ingress URL.
type StreamResponse struct {
	ID         int          `json:"id" example:"1"`
	Key        string       `json:"key" example:"a3f2c891-4b5e-4d7a-9c12-8f4e6d2a1b90"`
	Name       string       `json:"name" example:"Salary"`
	Status     StreamStatus `json:"status" enums:"idle,ready,live,paused,ended,failed" example:"idle"`
	IngressURL string       `json:"ingress_url" example:"rtmp://localhost:5554/live/a3f2c891-4b5e-4d7a-9c12-8f4e6d2a1b90"`
	MasterURL  string       `json:"master_url" example:"http://localhost:5555/hls/a3f2c891-4b5e-4d7a-9c12-8f4e6d2a1b90/master.m3u8"`
}

// IngressURL is the stream key paired with its ingress endpoint.
type IngressURL struct {
	Key string `json:"key" example:"a3f2c891-4b5e-4d7a-9c12-8f4e6d2a1b90"`
	URL string `json:"url" example:"rtmp://localhost:5554/live/a3f2c891-4b5e-4d7a-9c12-8f4e6d2a1b90"`
}

// IngressURLResponse is returned by the ingress URL endpoint.
type IngressURLResponse struct {
	Key string `json:"key" example:"a3f2c891-4b5e-4d7a-9c12-8f4e6d2a1b90"`
	URL string `json:"url" example:"rtmp://localhost:5554/live/a3f2c891-4b5e-4d7a-9c12-8f4e6d2a1b90"`
}

func StreamFromDB(s db.Stream) (Stream, error) {
	key, err := StreamKeyString(s.Key)
	if err != nil {
		return Stream{}, err
	}
	return Stream{
		ID:     int(s.ID),
		Key:    key,
		Name:   s.Name,
		Status: StreamStatus(s.Status),
	}, nil
}

func StreamsFromDB(rows []db.Stream) ([]Stream, error) {
	out := make([]Stream, 0, len(rows))
	for _, row := range rows {
		s, err := StreamFromDB(row)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func StreamKeyString(u pgtype.UUID) (string, error) {
	if !u.Valid {
		return "", fmt.Errorf("invalid stream key")
	}
	id, err := uuid.FromBytes(u.Bytes[:])
	if err != nil {
		return "", fmt.Errorf("invalid stream key: %w", err)
	}
	return id.String(), nil
}

func ToDBStreamStatus(status StreamStatus) (db.StreamStatus, error) {
	if status == "" {
		return "", nil
	}
	if !status.Valid() {
		return "", fmt.Errorf("invalid status %q", status)
	}
	return db.StreamStatus(status), nil
}

func StreamResponseFromDB(s db.Stream, ingressBase, publicURL string) (StreamResponse, error) {
	e, err := StreamFromDB(s)
	if err != nil {
		return StreamResponse{}, err
	}
	return StreamResponse{
		ID:         e.ID,
		Key:        e.Key,
		Name:       e.Name,
		Status:     e.Status,
		IngressURL: BuildIngressURL(ingressBase, e.Key),
		MasterURL:  strings.TrimRight(publicURL, "/") + "/hls/" + e.Key + "/master.m3u8",
	}, nil
}

func BuildIngressURL(base, key string) string {
	return strings.TrimRight(base, "/") + "/" + key
}
