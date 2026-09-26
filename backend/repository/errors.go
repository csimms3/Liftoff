package repository

import "errors"

// ErrTemplateNotFound is returned when a workout or routine template ID is unknown.
var ErrTemplateNotFound = errors.New("template not found")

// ErrInvalidSetIndex is returned when a set index is outside the exercise's sets.
var ErrInvalidSetIndex = errors.New("invalid set index")

// ErrNotFound is returned when a record doesn't exist or doesn't belong to the
// requesting user. The two are deliberately indistinguishable to callers.
var ErrNotFound = errors.New("not found")
