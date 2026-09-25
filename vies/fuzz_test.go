package vies

import (
	"net/http"
	"testing"
	"unicode/utf8"
)

func FuzzRetryAfter(f *testing.F) {
	for _, seed := range []string{"", "0", "30", "-1", "10000000000000000", "Wed, 21 Oct 2015 07:28:00 GMT", "soon"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, v string) {
		d := retryAfter(http.Header{"Retry-After": {v}})
		if d < 0 {
			t.Fatalf("negative delay %s for %q", d, v)
		}
	})
}

func FuzzErrorMessage(f *testing.F) {
	for _, seed := range [][]byte{nil, []byte(`{}`), []byte(`{"message":"MS_UNAVAILABLE"}`), []byte(`<html>`), []byte("\xff\xfe")} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		msg := errorMessage(body)
		if len(body) > 200 && !utf8.ValidString(msg) {
			t.Fatalf("invalid UTF-8 in truncated message %q", msg)
		}
		if len(body) > 200 && len(msg) > 200 {
			t.Fatalf("message not truncated: %d bytes", len(msg))
		}
	})
}
