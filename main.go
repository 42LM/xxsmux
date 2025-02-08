package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type XXSMux struct {
	patterns      map[string]http.Handler
	patternPrefix string
	middlewares   []Middleware
	root          *XXSMux
	parent        *XXSMux

	subXXSMux []*XXSMux
}

func NewXXSMux() *XXSMux {
	mux := &XXSMux{patterns: map[string]http.Handler{}}
	return mux
}

func removeDoubleSlash(text string) string {
	re := regexp.MustCompile(`//+`)
	return re.ReplaceAllString(text, "/")
}

// .Path
func (mux *XXSMux) Pattern(patterns map[string]http.Handler) {
	patternPrefix := mux.patternPrefix
	mux.patternPrefix = ""
	mux.patternPrefix = mux.root.patternPrefix + "/"

	for _, subMux := range mux.parent.subXXSMux {
		if mux.parent == mux.root {
			if subMux == mux {
				for _, subSubMux := range subMux.subXXSMux {
					mux.patternPrefix = subSubMux.patternPrefix + "/"
				}
			}
		} else {
			for _, subSubMux := range subMux.subXXSMux {
				mux.patternPrefix = subSubMux.patternPrefix + "/"
			}
		}
	}

	mux.patternPrefix += patternPrefix

	for pattern, handler := range patterns {
		// TODO: strings.Split could fail and not have 2 elements
		mux.patterns[removeDoubleSlash(mux.patternPrefix+strings.Split(pattern, " ")[1])] = handler
		fmt.Println("PATTTT:", mux.patterns)
	}
	mux.subXXSMux = append(mux.subXXSMux, mux)
}

func (mux *XXSMux) Use(middleware ...Middleware) {
	mux.middlewares = append(mux.middlewares, middleware...)
}

func (mux *XXSMux) Prefix(prefix string) {
	// TODO: validate prefix (check if first char is `/`)
	mux.patternPrefix = prefix
}

func (mux *XXSMux) Subrouter() *XXSMux {
	subMux := NewXXSMux()
	subMux.parent = mux
	subMux.root = mux.root

	if mux.root.middlewares != nil && subMux != mux.root {
		subMux.middlewares = append(subMux.middlewares, mux.root.middlewares...)
	}

	mux.subXXSMux = append(mux.subXXSMux, subMux)

	return subMux
}

func (mux *XXSMux) Build(defaultServeMux *http.ServeMux) {
	queue := []*XXSMux{mux}
	visited := make(map[*XXSMux]bool)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue // skip if already visited (prevents cycles)
		}
		visited[current] = true // Mark as visited

		// Process the current node
		if current.patterns != nil {
			for pattern, handler := range current.patterns {
				// pat[pattern] = handler
				defaultServeMux.Handle(pattern, NewHandler(current.middlewares...)(handler))
				fmt.Println("  Pattern:", pattern)
			}
		}

		// Enqueue children (sub-muxes)
		queue = append(queue, current.subXXSMux...)
	}
}

func main() {
	router := NewXXSMux()
	router.root = router
	router.parent = router
	router.Use(Middleware1, Middleware4)

	// /v1/test
	// /v1/a
	// /v1/b
	router.Prefix("v1")
	router.Pattern(map[string]http.Handler{
		"GET /test": http.HandlerFunc(greet),
		"GET /a":    http.HandlerFunc(greet),
		"GET /b":    http.HandlerFunc(greet),
	})

	// /v1/v2/{instance_id}/test
	v1Router := router.Subrouter()
	v1Router.Prefix("v2/{instance_id}")
	v1Router.Pattern(map[string]http.Handler{
		"GET /test": http.HandlerFunc(greet),
	})

	// /v1/v2/{instance_id}/foo
	v12Router := v1Router.Subrouter()
	v12Router.Use(Middleware3)
	// v12Router.Prefix("")
	v12Router.Pattern(map[string]http.Handler{
		"GET /foo": http.HandlerFunc(greet),
	})

	// /v1/v2/{instance_id}/foobar/foo
	v13Router := v12Router.Subrouter()
	v13Router.Use(Middleware3)
	v13Router.Prefix("foobar")
	v13Router.Pattern(map[string]http.Handler{
		"GET /bar": http.HandlerFunc(greet),
	})

	// /v1/boo/test
	v2Router := router.Subrouter()
	v2Router.Prefix("boo")

	v2Router.Pattern(map[string]http.Handler{
		"GET /test": http.HandlerFunc(greet),
	})
	v2Router.Use(Middleware2)

	// /v1/secret
	adminRouter := router.Subrouter()
	adminRouter.Use(AdminMiddleware)
	// adminRouter.Prefix("")
	adminRouter.Pattern(map[string]http.Handler{
		"GET /secret": http.HandlerFunc(greet),
	})

	defaultServeMux := http.DefaultServeMux

	// build the default serve mux aka
	// fill it with path patterns and the additional handlers
	router.Build(defaultServeMux)

	s := http.Server{
		Addr:    ":8080",
		Handler: defaultServeMux,
	}

	s.ListenAndServe()
}

type Middleware func(http.Handler) http.Handler

func NewHandler(mw ...Middleware) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		next := h
		for _, m := range mw {
			next = m(next)
		}
		return next
	}
}

func greet(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("url.Path: %v\n", r.URL.Path)
	fmt.Printf("url.RawPath: %v\n", r.URL.RawPath)
	fmt.Printf("url.EscapedPath(): %v\n", r.URL.EscapedPath())
	name := r.PathValue("name")
	fmt.Fprintf(w, "Hello %s", name)
}

func helloWorld(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("url.Path: %v\n", r.URL.Path)
	fmt.Printf("url.RawPath: %v\n", r.URL.RawPath)
	fmt.Printf("url.EscapedPath(): %v\n", r.URL.EscapedPath())
	for range 7 {
		fmt.Fprint(w, "Hello world")
	}
}

func secret(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("url.Path: %v\n", r.URL.Path)
	fmt.Printf("url.RawPath: %v\n", r.URL.RawPath)
	fmt.Printf("url.EscapedPath(): %v\n", r.URL.EscapedPath())
	fmt.Fprintln(w, "Beep Boop Bob hello agent")
}

func Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func Middleware1(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "HELLO FROM MIDDLEWARE #1")

		next.ServeHTTP(w, r)
	})
}

func Middleware2(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "HELLO FROM MIDDLEWARE #2")

		next.ServeHTTP(w, r)
	})
}

func Middleware3(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "HELLO FROM MIDDLEWARE #3")

		next.ServeHTTP(w, r)
	})
}

func Middleware4(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "HELLO FROM MIDDLEWARE #4")

		next.ServeHTTP(w, r)
	})
}

func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "HELLO FROM ADMIN MIDDLEWARE")

		usr, pw, ok := r.BasicAuth()
		if !ok {
			fmt.Fprintln(w, "⚠️ RESTRICTED AREA")
			return
		}
		if usr == "007" && pw == "martini" {
			next.ServeHTTP(w, r)
		} else {
			fmt.Fprintln(w, "AGENT WHO??? 🤣")
			return
		}
	})
}

func Chain(base http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for _, m := range middleware {
		base = m(base)
	}
	return base
}

func ChainRouter(base http.Handler, handlers ...http.Handler) http.Handler {
	finalHandler := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r) // Call the next handler
		})
	}

	for _, handler := range handlers {
		base = finalHandler(handler)
	}

	return base
}
