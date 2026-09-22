package api

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestError(t *testing.T) {
	t.Run("code accessor", func(t *testing.T) {
		err := ErrServer.WithCode("503").WithMessage("MS_UNAVAILABLE")
		assert.Equal(t, "503", err.Code())
		assert.Equal(t, "server: 503: MS_UNAVAILABLE", err.Error())
		assert.Empty(t, ErrServer.Code())
	})

	t.Run("sentinels stay distinct", func(t *testing.T) {
		err := ErrServer.WithCode("500")
		assert.ErrorIs(t, err, ErrServer)
		assert.NotErrorIs(t, err, ErrNetwork)
		assert.NotErrorIs(t, err, ErrInput)
	})

	t.Run("with methods copy rather than mutate", func(t *testing.T) {
		base := ErrNetwork
		derived := base.WithCode("502").WithMessage("bad gateway")
		assert.Empty(t, base.Code())
		assert.Equal(t, "502", derived.Code())
	})

	t.Run("cause is unwrapped", func(t *testing.T) {
		cause := fmt.Errorf("dial tcp: refused")
		err := ErrNetwork.WithCause(cause)
		assert.ErrorIs(t, err, cause)
		assert.Equal(t, cause, errors.Unwrap(err))
	})

	t.Run("rate limited carries the 429 code", func(t *testing.T) {
		err := &RateLimitedError{}
		assert.Equal(t, "429", err.Code())
	})

	t.Run("errors.As finds the type", func(t *testing.T) {
		var e *Error
		wrapped := fmt.Errorf("supplier: %w", ErrServer.WithCode("500"))
		require.True(t, errors.As(wrapped, &e))
		assert.Equal(t, "500", e.Code())
	})
}
