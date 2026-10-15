package enformion

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Literals the internal tests repeat.
const (
	jsonNull   = "null"
	personKey  = "person"
	isErrorKey = "isError"
)

// asCallError returns err as a *CallError, failing tb otherwise.
func asCallError(tb testing.TB, err error) *CallError {
	tb.Helper()
	ce, ok := errors.AsType[*CallError](err)
	require.True(tb, ok, "want a *CallError, got %T (%v)", err, err)
	return ce
}

// assertValidation asserts that err is a *ValidationError matching sentinel,
// with field and reason, whose text is the fixed grammar built from them alone.
func assertValidation(tb testing.TB, err, sentinel error, field, reason string) {
	tb.Helper()
	require.ErrorIs(tb, err, sentinel)
	ve, ok := errors.AsType[*ValidationError](err)
	require.True(tb, ok, "want a *ValidationError, got %T", err)
	assert.Equal(tb, field, ve.Field)
	assert.Equal(tb, reason, ve.Reason)
	assert.Equal(tb, sentinel.Error()+": "+field+": "+reason, err.Error())
}
