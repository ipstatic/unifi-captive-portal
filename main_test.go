package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func init() {
	c := "testdata/config.yml"
	configFile = &c

	var err error
	config, err = loadConfiguration("testdata/config.yml")
	if err != nil {
		panic("Could not load test configuration: " + err.Error())
	}
}

func TestLandingHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/?id=1234&ap=ap01&ssid=ssid", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(landingHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := "Test Captive Portal"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}

}

func TestLandingHandlerValidation(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(landingHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	expected := "An error has occured"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}

}

func TestFormHandlerValidation(t *testing.T) {
	data := make(url.Values)
	data.Set("email", "user@example.com")
	data.Set("id", "1234")
	data.Set("ap", "ap01")
	data.Set("ssid", "ssid")
	data.Set("url", "")

	req, err := http.NewRequest("POST", "/form", strings.NewReader(data.Encode()))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(formHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := "You must agree to the Terms of Service"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}

}

func TestAuthUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"response":"OK"}`)
	}))
	defer ts.Close()

	config.UnifiURL = ts.URL
	err := authUser("02:00:00:00:00:01", "ap01")
	if err != nil {
		t.Errorf("Mock request to UniFi controller returned: %v", err)
	}

	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprintln(w, `{"response":"OK"}`)
	}))
	defer ts.Close()

	config.UnifiURL = ts.URL
	err = authUser("02:00:00:00:00:01", "ap01")
	expected := "Controller returned non 200 status code"

	if !strings.Contains(err.Error(), expected) {
		t.Errorf("Mock request to UniFi controller returned: %v, wanted Controller returned non 200 status code", err)
	}
}
