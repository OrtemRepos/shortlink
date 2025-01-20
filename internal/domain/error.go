package domain

import "errors"

var ErrURLNotFound = errors.New("URL not found")
var ErrURLAlreadyExists = errors.New("URL already exists")
