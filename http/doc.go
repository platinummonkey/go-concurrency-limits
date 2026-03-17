// Package http provides HTTP server middleware and client RoundTripper
// implementations that enforce adaptive concurrency limits.
//
// Server usage:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/api", myHandler)
//
//	middleware := clhttp.NewServerMiddleware(
//	    clhttp.WithName("my-server"),
//	    clhttp.WithLimiter(myLimiter),
//	)
//	http.ListenAndServe(":8080", middleware(mux))
//
// Client usage:
//
//	client := &http.Client{
//	    Transport: clhttp.NewClientRoundTripper(
//	        clhttp.WithName("my-client"),
//	    ),
//	}
package http
