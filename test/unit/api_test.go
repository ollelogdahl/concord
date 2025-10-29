package unit_test

import (
	"testing"

	"github.com/ollelogdahl/concord"
	"github.com/stretchr/testify/assert"
)

func TestGetName(t *testing.T) {
	config := concord.Config{
		Name: "foo",
	}

	instance := concord.New(config)

	assert.Equal(t, instance.Name(), "foo")
}

func TestGetAddress(t *testing.T) {
	config := concord.Config{
		AdvAddr: "localhost:1234",
	}

	instance := concord.New(config)

	assert.Equal(t, instance.Address(), "localhost:1234")
}
