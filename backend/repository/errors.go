package repository

import "errors"

// ErrTemplateNotFound is returned when a workout or routine template ID is unknown.
var ErrTemplateNotFound = errors.New("template not found")
