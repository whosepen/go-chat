package service

import "errors"

var (
	ErrInvalidPassword = errors.New("username or password error")

	ErrGenerateToken = errors.New("generate token failed")

	ErrOccupiedUsername = errors.New("username is already exists")

	ErrInvalidID = errors.New("input ID error")
)
