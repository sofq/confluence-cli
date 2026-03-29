package oauth2

import (
	"net"
	"time"
)

// SetTokenEndpoint overrides the OAuth2 client-credentials token endpoint for tests.
func SetTokenEndpoint(url string) { tokenEndpoint = url }

// SetTokenEndpointThreeLO overrides the OAuth2 3LO token endpoint for tests.
func SetTokenEndpointThreeLO(url string) { tokenEndpointThreeLO = url }

// SetCallbackTimeout overrides the browser callback timeout for tests.
func SetCallbackTimeout(d time.Duration) { callbackTimeout = d }

// SetListenFunc overrides the TCP listener creation for tests.
func SetListenFunc(f func() (net.Listener, error)) { listenFunc = f }
