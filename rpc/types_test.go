package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestT18_RouteActionConstants(t *testing.T) {
	assert.Equal(t, RouteAction("show"), Show)
	assert.Equal(t, RouteAction("add"), Add)
	assert.Equal(t, RouteAction("del"), Del)
}

func TestT18_RouteActionConstants_StringValues(t *testing.T) {
	assert.Equal(t, "show", string(Show))
	assert.Equal(t, "add", string(Add))
	assert.Equal(t, "del", string(Del))
}

func TestT18_RouteActionConstants_AreDistinct(t *testing.T) {
	assert.NotEqual(t, Show, Add)
	assert.NotEqual(t, Add, Del)
	assert.NotEqual(t, Show, Del)
}
