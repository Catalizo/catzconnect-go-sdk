package catzconnect

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckBaseURL(t *testing.T) {
	for _, u := range []string{"https://api.example.com", "http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080", "http://10.0.2.2:8080"} {
		if err := CheckBaseURL(u, false); err != nil {
			t.Errorf("%s refused: %v", u, err)
		}
	}
	for _, u := range []string{"http://api.example.com", "http://localhost.example.com", "ftp://api.example.com", "api.example.com"} {
		err := CheckBaseURL(u, false)
		var ce *Error
		if !errors.As(err, &ce) || ce.Kind != KindValidation {
			t.Errorf("%s accepted or wrong error: %v", u, err)
		}
	}
	if err := CheckBaseURL("http://api.example.com", true); err != nil {
		t.Errorf("opt-in refused: %v", err)
	}
}

func TestSendRefusesPlainHTTPBeforeSending(t *testing.T) {
	t.Setenv("CATZCONNECT_BASE_URL", "http://api.example.com")
	t.Setenv("CATZCONNECT_ALLOW_INSECURE_HTTP", "")
	_, _, err := config(&EnvValues{APIKey: "live_k"}, false)
	if err == nil || !strings.Contains(err.Error(), "plain http://") {
		t.Fatalf("err = %v", err)
	}
	if _, base, err := config(&EnvValues{APIKey: "live_k"}, true); err != nil || base != "http://api.example.com" {
		t.Fatalf("opt-in: %q %v", base, err)
	}
	t.Setenv("CATZCONNECT_ALLOW_INSECURE_HTTP", "1")
	if _, _, err := config(&EnvValues{APIKey: "live_k"}, false); err != nil {
		t.Fatalf("env opt-in: %v", err)
	}
}

func TestWithAllowInsecureHTTP(t *testing.T) {
	if New().http.allowInsecureHTTP {
		t.Fatal("default must refuse")
	}
	if !New(WithAllowInsecureHTTP()).http.allowInsecureHTTP {
		t.Fatal("option not applied")
	}
	// A local httptest server (http://127.0.0.1) stays allowed without the option.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	if err := CheckBaseURL(srv.URL, false); err != nil {
		t.Fatal(err)
	}
}

func TestEnvValuesRedacted(t *testing.T) {
	e := EnvValues{APIKey: "live_abcdefghijklmnopqrstuvwxyz", PrivateKey: "PRIVATEKEYSECRET==", ServerPublicKey: "SERVERPUB=="}
	type wrapper struct{ Env EnvValues }
	for _, s := range []string{
		fmt.Sprintf("%v", e), fmt.Sprintf("%+v", e), fmt.Sprintf("%#v", e), fmt.Sprintf("%s", e),
		fmt.Sprintf("%v", &e), fmt.Sprintf("%+v", wrapper{e}), fmt.Sprint(e), e.String(), e.GoString(),
	} {
		if strings.Contains(s, "abcdefghijklmnopqrstuvwxyz") || strings.Contains(s, "PRIVATEKEYSECRET") {
			t.Errorf("secret printed: %s", s)
		}
		if !strings.Contains(s, "SERVERPUB==") {
			t.Errorf("public key missing: %s", s)
		}
	}
}
