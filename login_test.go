package main

import (
	"net/url"
	"testing"
)

func TestCallbackListenAddr(t *testing.T) {
	u, err := url.Parse("http://localhost:8080/oauth")
	if err != nil {
		t.Fatal(err)
	}

	// Outside Docker, bind the redirect URI's host so the callback stays on
	// loopback.
	t.Setenv(dockerEnvVar, "")
	if got := callbackListenAddr(u); got != "localhost:8080" {
		t.Errorf("outside Docker: got %q, want localhost:8080", got)
	}

	// Inside Docker, bind all interfaces on the same port so a published port
	// can reach the callback server.
	t.Setenv(dockerEnvVar, "1")
	if got := callbackListenAddr(u); got != "0.0.0.0:8080" {
		t.Errorf("in Docker: got %q, want 0.0.0.0:8080", got)
	}
}
