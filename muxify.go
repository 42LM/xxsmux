// Package muxify implements functionality for building a http.ServeMux.
//
// The muxify package is a default serve mux builder.
// Build patterns, handlers and wrap middlewares conveniently upfront.
// The muxify.Mux acts as a builder for the http.ServeMux.
// The overall goal of this package is to build the http.ServeMux with pattern/path prefixes and middleware wired in.
package muxify

import (
	"fmt"
	"net/http"
	"strings"
)

// Mux is a simple wrapper for the http.ServeMux.
type Mux struct {
	muxify             *http.ServeMux
	patternPrefix      string
	middlewares        []Middleware
	registeredPatterns *[]string
}

// Middleware represents an http.Handler wrapper to inject additional functionality.
type Middleware func(http.Handler) http.Handler

// NewMux returns a new muxify.Mux.
// This is a simple wrapper for the http.ServeMux.
func NewMux() *Mux {
	s := make([]string, 0)
	return &Mux{
		muxify:             http.NewServeMux(),
		registeredPatterns: &s,
	}
}

// Handle wraps the http.Handle func.
// It wraps the pattern with prefixes
// and the handler with middlewares.
func (mux *Mux) Handle(pattern string, handler http.Handler) {
	method, patternPath := splitPattern(pattern)
	pattern = method + mux.patternPrefix + patternPath
	mux.muxify.Handle(
		pattern,
		newHandler(mux.middlewares...)(handler),
	)
	*mux.registeredPatterns = append(*mux.registeredPatterns, pattern)
}

// HandleFunc wraps the http.HandleFunc func.
// It wraps the pattern with prefixes
// and the handler with middlewares.
func (mux *Mux) HandleFunc(pattern string, handlerFunc func(http.ResponseWriter, *http.Request)) {
	method, patternPath := splitPattern(pattern)
	pattern = method + mux.patternPrefix + patternPath
	mux.muxify.Handle(
		pattern,
		newHandler(mux.middlewares...)(http.HandlerFunc(handlerFunc)),
	)
	*mux.registeredPatterns = append(*mux.registeredPatterns, pattern)
}

// Subrouter returns a sub mux.
func (mux *Mux) Subrouter() *Mux {
	return &Mux{
		muxify:             mux.muxify,
		patternPrefix:      mux.patternPrefix,
		middlewares:        mux.middlewares,
		registeredPatterns: mux.registeredPatterns,
	}
}

// Use wraps a middleware to the mux.
func (mux *Mux) Use(middleware ...Middleware) {
	mux.middlewares = append(mux.middlewares, middleware...)
}

// Prefix sets a prefix for the mux.
func (mux *Mux) Prefix(prefix string) *Mux {
	if len(prefix) > 0 {
		if prefix[0] != '/' {
			prefix = "/" + prefix
		}
	}

	mux.patternPrefix += prefix
	return mux
}

// PrintRegisteredPatterns prints the registered patterns of the http.ServeMux.
// The Build() method needs to be called before!
func (mux *Mux) PrintRegisteredPatterns() {
	fmt.Println("* Registered patterns:", strings.Repeat("*", 47))
	fmt.Println(strings.Join(*mux.registeredPatterns, "\n"))
	fmt.Printf("%s\n\n", strings.Repeat("*", 70))
}

// Implement http.Handler interface.
func (mux *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux.muxify.ServeHTTP(w, r)
}

// newHandler returns an http.Handler wrapped with given middlewares.
func newHandler(mw ...Middleware) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		for i := len(mw) - 1; i >= 0; i-- {
			h = mw[i](h)
		}
		return h
	}
}

// splitPattern helps splitting the pattern "GET /a/b"
// more specifically the method from the path and
// returns both as a string.
func splitPattern(pattern string) (method string, patternPath string) {
	splitPattern := strings.Split(pattern, " ")

	switch len(splitPattern) {
	case 2:
		method = splitPattern[0] + " "
		patternPath = splitPattern[1]
	default:
		patternPath = splitPattern[0]
	}

	return method, patternPath
}
