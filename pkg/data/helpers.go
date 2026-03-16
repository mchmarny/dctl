package data

import "errors"

// ErrDBNotInitialized is returned when the database has not been opened.
var ErrDBNotInitialized = errors.New("database not initialized")

// Contains checks for val in list
func Contains[T comparable](list []T, val T) bool {
	if list == nil {
		return false
	}
	for _, item := range list {
		if item == val {
			return true
		}
	}
	return false
}
