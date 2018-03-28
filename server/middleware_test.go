package main

import (
	"net/http"
	"testing"
)

func emptyTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		return
	})
}

func TestServiceLoader(t *testing.T) {
	_ = serviceLoader(emptyTestHandler())
}
