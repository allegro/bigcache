package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func emptyTestHandler() service {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		})
	}
}

func TestServiceLoader(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/v1/stats", nil)
	if err != nil {
		t.Error(err)
	}
	rr := httptest.NewRecorder()
	testHandlers := serviceLoader(cacheIndexHandler(), emptyTestHandler())
	testHandlers.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusAccepted {
		t.Errorf("handlers not loading properly. want: 202, got: %d", rr.Code)
	}
}
