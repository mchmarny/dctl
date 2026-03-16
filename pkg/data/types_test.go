package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventSearchCriteria_String(t *testing.T) {
	q := EventSearchCriteria{PageSize: 10, Page: 1}
	s := q.String()
	assert.Contains(t, s, "page_size")
}
