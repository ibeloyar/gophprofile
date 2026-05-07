package model

import "errors"

var (
	ErrForbidden      = errors.New("forbidden")
	ErrAvatarNotFound = errors.New("avatar not found")
)
