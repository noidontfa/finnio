package entity

import "strings"

// MediaMTXAuthRequest is the payload sent by MediaMTX for HTTP authentication.
type MediaMTXAuthRequest struct {
	User      string `json:"user" validate:"required"`
	Password  string `json:"password" validate:"required"`
	Token     string `json:"token" validate:"required"`
	IP        string `json:"ip"`
	Action    string `json:"action" validate:"required"`
	Path      string `json:"path" validate:"required"`
	Protocol  string `json:"protocol"`
	ID        string `json:"id" validate:"required"`
	Query     string `json:"query"`
	UserAgent string `json:"userAgent" validate:"required"`
}

// MediaMTXHookRequest is the payload for a MediaMTX lifecycle hook.
type MediaMTXHookRequest struct {
	Event string `json:"event" validate:"required"`
	Path  string `json:"path" validate:"required"`
}

var abrSuffixes = []string{"_360p", "_480p", "_720p", "_1080p"}

// BaseStreamPath maps ABR ladder paths back to the registered stream key.
func BaseStreamPath(path string) string {
	for _, suffix := range abrSuffixes {
		if strings.HasSuffix(path, suffix) {
			return strings.TrimSuffix(path, suffix)
		}
	}
	return path
}

// IsABRPath reports whether path is an ABR republish variant, not the source stream.
func IsABRPath(path string) bool {
	return BaseStreamPath(path) != path
}

func (r MediaMTXAuthRequest) StreamKey() string {
	return BaseStreamPath(r.Path)
}

func (r MediaMTXHookRequest) StreamKey() string {
	return BaseStreamPath(r.Path)
}
