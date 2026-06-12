package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicAuth(t *testing.T) {
	wrapped := BasicAuth("admin", "s3cret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name       string
		user, pass string
		creds      bool
		want       int
	}{
		{"no credentials", "", "", false, http.StatusUnauthorized},
		{"wrong user", "nope", "s3cret", true, http.StatusUnauthorized},
		{"wrong password", "admin", "nope", true, http.StatusUnauthorized},
		{"correct", "admin", "s3cret", true, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/state", nil)
			if tc.creds {
				req.SetBasicAuth(tc.user, tc.pass)
			}
			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Errorf("status = %d, want %d", w.Code, tc.want)
			}
			if tc.want == http.StatusUnauthorized && w.Header().Get("WWW-Authenticate") == "" {
				t.Error("401 must include WWW-Authenticate header")
			}
		})
	}
}
