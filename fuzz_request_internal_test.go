package enformion

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzRequestEncode(f *testing.F) {
	full := rqFull()
	f.Add(full.FirstName, full.MiddleName, full.LastName, full.DOB, full.Age,
		full.AddressLine1, full.AddressLine2, full.Phone, full.Email, full.SessionID)
	f.Add("Avery", "", "", "", 0, "", "", "312-555-0142", "", "")
	f.Add("A<b>&c", "", "O\"Brien\\", "02/29/2000", 150, "", "45000", "", "x&y@example.com", "")
	f.Add("\xff", "\t", " ", "1/2/1990", -1, "\x00", "\u0085", "\n", "\x7f", "has space")
	f.Add("Zoë", "", "Ñúñez", "13/01/2020", 151, "\u3000", "", "", "", "séance")
	f.Fuzz(func(t *testing.T, first, middle, last, dob string, age int, line1, line2, phone, email, session string) {
		r := EnrichContactRequest{
			FirstName: first, MiddleName: middle, LastName: last, DOB: dob, Age: age,
			AddressLine1: line1, AddressLine2: line2, Phone: phone, Email: email, SessionID: session,
		}
		verr := r.validate()
		body, err := r.encode()
		require.Equal(t, verr == nil, err == nil)
		if err != nil {
			require.Nil(t, body)
			ve, ok := errors.AsType[*ValidationError](err)
			require.True(t, ok)
			require.ErrorIs(t, err, ErrInvalidRequest)
			fields := []string{"FirstName", "MiddleName", "LastName", "DOB", "Age", "AddressLine1", "AddressLine2", "Phone", "Email", "SessionID", "request"}
			reasons := []string{
				reasonInvalidUTF8, reasonControlCharacter, reasonBlank, reasonTooLong, reasonInvalidFormat,
				reasonOutOfRange, reasonInvalidHeaderValue, reasonInsufficientCategories,
			}
			require.True(t, slices.Contains(fields, ve.Field), "unknown field %q", ve.Field)
			require.True(t, slices.Contains(reasons, ve.Reason), "unknown reason %q", ve.Reason)
			// The text follows a fixed grammar, so it can never quote a value.
			require.Equal(t, "enformion: invalid request: "+ve.Field+": "+ve.Reason, err.Error())
			return
		}

		require.True(t, json.Valid(body))
		require.False(t, bytes.HasSuffix(body, []byte("\n")))
		dec := json.NewDecoder(bytes.NewReader(body))
		var w wireRequest
		require.NoError(t, dec.Decode(&w))
		_, terr := dec.Token()
		require.ErrorIs(t, terr, io.EOF, "the body is exactly one JSON value")
		require.Equal(t, '{', rune(body[0]))

		require.Equal(t, first, w.FirstName)
		require.Equal(t, middle, w.MiddleName)
		require.Equal(t, last, w.LastName)
		require.Equal(t, dob, w.Dob)
		require.Equal(t, age, w.Age)
		require.Equal(t, phone, w.Phone)
		require.Equal(t, email, w.Email)
		if line1 == "" && line2 == "" {
			require.Nil(t, w.Address)
		} else {
			require.NotNil(t, w.Address)
			require.Equal(t, line1, w.Address.AddressLine1)
			require.Equal(t, line2, w.Address.AddressLine2)
		}
	})
}
