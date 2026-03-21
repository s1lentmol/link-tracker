package apperr

import "errors"

var (
	ErrChatExists      = errors.New("chat already exists")
	ErrChatNotFound    = errors.New("chat not found")
	ErrLinkExists      = errors.New("link already tracked")
	ErrLinkNotFound    = errors.New("link not tracked")
	ErrTagExists       = errors.New("tag already exists")
	ErrTagNotFound     = errors.New("tag not found")
	ErrUnsupportedLink = errors.New("unsupported link")
	ErrInvalidLink     = errors.New("invalid link")
	ErrInvalidRequest  = errors.New("invalid request")
)
