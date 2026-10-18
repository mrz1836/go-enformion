package enformion

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
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

// fqSearchField matches every field a Person Search ValidationError may name.
var fqSearchField = regexp.MustCompile(`^(Mode|FirstName|MiddleName|LastName|DOB|Age|AgeRangeMinAge|AgeRangeMaxAge|` +
	`AgeRange|Email|Phone|ClientIP|FirstNameCharOffset|LastNameCharOffset|DOBFormat|MaxAddressYears|MaxPhoneYears|` +
	`Page|ResultsPerPage|SessionID|request|` +
	`(AKAs|Relatives)(\[\d+\](\.(FirstName|MiddleName|LastName))?)?|` +
	`Addresses(\[\d+\](\.(AddressLine1|AddressLine2|County))?)?|` +
	`(TahoeIDs|Includes|FilterOptions)(\[\d+\])?)$`)

func FuzzPersonSearchRequestEncode(f *testing.F) {
	full := sqFull()
	f.Add(string(full.Mode), full.FirstName, full.AKAs[0].LastName, full.DOB, full.Age, full.AgeRange,
		full.Addresses[0].County, full.Email, full.Phone, full.ClientIP, full.TahoeIDs[0],
		string(full.Includes[0]), string(full.FilterOptions[0]), 3, full.Page, full.ResultsPerPage, full.SessionID)
	f.Add("Teaser", "", "", "", 0, "", "", "", "", "", "", "", "", -1, 0, 0, "")
	f.Add("ReversePhonePerson", "", "", "", 0, "", "", "", "(312) 555-0142", "", "", "", "", -1, 1, 1000, "")
	f.Add("Person", "A<b>&c", "O\"Brien\\", "02/29/2000", 150, "0-150", "Zoë", "x&y@example.com", "", "192.0.2.255",
		"G-1", "Phone Numbers", "a-b", 256, 1<<31-1, 1001, "s")
	f.Add("teaser", "\xff", "\t", "1/2/1990", -1, "30-40-50", " ", "\x00", "\u0085", "192.0.2.010", "", "", "", 257, -1, -1, "has space")
	f.Fuzz(func(t *testing.T, mode, first, aka, dob string, age int, ageRange, county, email, phone, clientIP, tahoe,
		include, filter string, offset, page, perPage int, session string,
	) {
		r := PersonSearchRequest{
			Mode: PersonSearchMode(mode), FirstName: first, DOB: dob, Age: age, AgeRange: ageRange,
			Email: email, Phone: phone, ClientIP: clientIP, Page: page, ResultsPerPage: perPage, SessionID: session,
		}
		if aka != "" {
			r.AKAs = []PersonSearchRequestName{{LastName: aka}}
		}
		if county != "" {
			r.Addresses = []PersonSearchRequestAddress{{County: county}}
		}
		if tahoe != "" {
			r.TahoeIDs = []string{tahoe}
		}
		if include != "" {
			r.Includes = []PersonSearchInclude{PersonSearchInclude(include)}
		}
		if filter != "" {
			r.FilterOptions = []PersonSearchFilter{PersonSearchFilter(filter)}
		}
		if offset >= 0 {
			r.FirstNameCharOffset = &offset
		}
		before := r
		verr := r.validate()
		body, err := r.encode()
		require.Equal(t, verr == nil, err == nil)
		require.Equal(t, before, r, "validation never modifies the request")
		if err != nil {
			require.Nil(t, body)
			ve, ok := errors.AsType[*ValidationError](err)
			require.True(t, ok)
			require.ErrorIs(t, err, ErrInvalidRequest)
			reasons := []string{
				reasonInvalidUTF8, reasonControlCharacter, reasonBlank, reasonTooLong, reasonInvalidFormat,
				reasonOutOfRange, reasonInvalidHeaderValue, reasonRequired, reasonUnsupported, reasonTooMany, reasonNoCriteria,
			}
			require.Regexp(t, fqSearchField, ve.Field)
			require.True(t, slices.Contains(reasons, ve.Reason), "unknown reason %q", ve.Reason)
			// The text follows a fixed grammar, so it can never quote a value.
			require.Equal(t, "enformion: invalid request: "+ve.Field+": "+ve.Reason, err.Error())
			return
		}

		require.True(t, json.Valid(body))
		require.False(t, bytes.HasSuffix(body, []byte("\n")))
		dec := json.NewDecoder(bytes.NewReader(body))
		var w wirePersonSearch
		require.NoError(t, dec.Decode(&w))
		_, terr := dec.Token()
		require.ErrorIs(t, terr, io.EOF, "the body is exactly one JSON value")
		require.Equal(t, '{', rune(body[0]))
		var keys map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(body, &keys))
		require.NotContains(t, keys, "Mode")
		require.NotContains(t, keys, "SessionID")

		require.Equal(t, first, w.FirstName)
		require.Equal(t, dob, w.Dob)
		require.Equal(t, age, w.Age)
		require.Equal(t, ageRange, w.AgeRange)
		require.Equal(t, email, w.Email)
		require.Equal(t, phone, w.Phone)
		require.Equal(t, clientIP, w.ClientIP)
		require.Equal(t, page, w.Page)
		require.Equal(t, perPage, w.ResultsPerPage)
		require.Equal(t, wireNames(r.AKAs), w.Akas)
		require.Equal(t, r.TahoeIDs, w.TahoeIDs)
		require.Equal(t, r.Includes, w.Includes)
		require.Equal(t, r.FilterOptions, w.FilterOptions)
		require.Equal(t, r.FirstNameCharOffset, w.FirstNameCharOffset)
		if county == "" {
			require.Nil(t, w.Addresses)
		} else {
			require.Equal(t, []wirePersonSearchAddress{{County: county}}, w.Addresses)
		}
	})
}
