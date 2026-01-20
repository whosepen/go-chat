package service

import "errors"

var ErrInvalidPassword = errors.New("username or password error")

var ErrGenerateToken = errors.New("generate token failed")

var ErrOccupiedUsername = errors.New("username is already exists")

var ErrInvalidID = errors.New("input ID error")
