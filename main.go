package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"

	log "github.com/sirupsen/logrus"
)

var (
	configFile = flag.String(
		"config.file", "unifi-portal.yml",
		"UniFi captive portal configuration file.",
	)
	listenAddress = flag.String(
		"web.listen-address", ":4646",
		"Address to listen on for requests.",
	)
	templateDir = flag.String(
		"template.dir", "templates",
		"Directory which contains HTML templates.",
	)
	assetDir = flag.String(
		"asset.dir", "assets",
		"Directory which contains css/js/img assets.",
	)
	verbose = flag.Bool(
		"verbose", false,
		"Enable verbose/debug logging.",
	)
	version = flag.Bool(
		"version", false,
		"Print version/build information.",
	)

	templates = template.Must(template.ParseGlob(fmt.Sprintf("%s/*", *templateDir)))
	config    *Config

	Version string
	Commit  string
	Branch  string
)

// Config represents the YAML structure of the configuration file.
type Config struct {
	// UnifiURL URL of the UniFi instance you want to register users against
	UnifiURL string `yaml:"unifi_url"`
	// UnifiUsername username for UniFi API
	UnifiUsername string `yaml:"unifi_username"`
	// UnifiPassword password for UniFi API
	UnifiPassword string `yaml:"unifi_password"`
	// UnifiSite site for UniFi Controller
	UnifiSite string `yaml:"unifi_site"`
	// Title of HTML pages
	Title string `yaml:"title"`
	// TOS Terms of Service
	TOS string `yaml:"tos"`
	// Intro text on form page
	Intro string `yaml:"intro"`
	// Minutes user is authenticate for in one session
	Minutes string `yaml:"minutes"`
	// RedirectURL url to redirect user to if they did not have one supplied
	RedirectURL string `yaml:"redirect_url"`
	// DynamoTableName
	DynamoTableName string `yaml:"dynamo_table_name"`
}

type Db struct {
	Email string    `json:"email"`
	ID    string    `json:"id"`
	AP    string    `json:"ap"`
	SSID  string    `json:"ssid"`
	Date  time.Time `json:"date"`
}

type Page struct {
	Title     string
	ID        string
	AP        string
	SSID      string
	URL       string
	TOS       string
	Intro     string
	Errors    map[string]string
	FormEmail string
	FormTOS   string
}

func authUser(id string, ap string) error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Jar: jar, Transport: tr}

	data := map[string]string{
		"username": config.UnifiUsername,
		"password": config.UnifiPassword,
	}

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(data)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/login", config.UnifiURL), b)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"code": resp.StatusCode,
	}).Debug("login response")

	if resp.StatusCode != 200 {
		return errors.New("Controller returned non 200 status code")
	}

	data = map[string]string{
		"cmd":     "authorize-guest",
		"mac":     id,
		"minutes": config.Minutes,
	}

	b = new(bytes.Buffer)
	json.NewEncoder(b).Encode(data)

	req, err = http.NewRequest("POST", fmt.Sprintf("%s/api/s/%s/cmd/stamgr", config.UnifiURL, config.UnifiSite), b)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"code": resp.StatusCode,
	}).Debug("authorize command response")

	if resp.StatusCode != 200 {
		return errors.New("Controller returned non 200 status code")
	}

	data = map[string]string{
		"cmd":     "authorize-guest",
		"mac":     id,
		"minutes": config.Minutes,
	}

	req, err = http.NewRequest("GET", fmt.Sprintf("%s/logout", config.UnifiURL), nil)
	_, err = client.Do(req)
	if err != nil {
		return err
	}

	return nil
}

func dbSave(email string, id string, ap string, ssid string) error {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := dynamodb.New(sess)

	item := Db{
		Email: email,
		ID:    id,
		AP:    ap,
		SSID:  ssid,
		Date:  time.Now(),
	}

	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(config.DynamoTableName),
	}

	_, err = svc.PutItem(input)
	if err != nil {
		return err
	}

	return nil
}

func landingHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		log.Error("Request missing id query parameter")
		errorHandler(w, r, http.StatusBadRequest)
		return
	}

	ap := r.URL.Query().Get("ap")
	if ap == "" {
		log.Error("Request missing ap query parameter")
		errorHandler(w, r, http.StatusBadRequest)
		return
	}

	ssid := r.URL.Query().Get("ssid")
	if ssid == "" {
		log.Error("Request missing ssid query parameter")
		errorHandler(w, r, http.StatusBadRequest)
		return
	}

	url := r.URL.Query().Get("url")

	vars := Page{
		Title: config.Title,
		ID:    id,
		AP:    ap,
		SSID:  ssid,
		URL:   url,
		TOS:   config.TOS,
		Intro: config.Intro,
	}

	err := templates.ExecuteTemplate(w, "landingPage", vars)
	if err != nil {
		log.Error(err.Error())
		errorHandler(w, r, http.StatusInternalServerError)
		return
	}
}

func formHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	errors := make(map[string]string)

	if len(r.Form["email"][0]) == 0 {
		errors["email"] = "Please enter your email address"
	}

	var tos string
	_, ok := r.Form["tos"]
	if ok {
		tos = "on"
	} else {
		errors["tos"] = "You must agree to the Terms of Service"
	}

	if len(errors) != 0 {
		log.WithFields(log.Fields{
			"errors": errors,
		}).Debug("Form Errors")

		vars := Page{
			Title:     config.Title,
			ID:        r.Form["id"][0],
			AP:        r.Form["ap"][0],
			SSID:      r.Form["ssid"][0],
			URL:       r.Form["url"][0],
			TOS:       config.TOS,
			Intro:     config.Intro,
			Errors:    errors,
			FormEmail: r.Form["email"][0],
			FormTOS:   tos,
		}

		err := templates.ExecuteTemplate(w, "landingPage", vars)
		if err != nil {
			log.Error(err.Error())
			errorHandler(w, r, http.StatusInternalServerError)
			return
		}
		return
	}

	err := authUser(r.Form["id"][0], r.Form["ap"][0])
	if err != nil {
		log.Error(err.Error())
		errorHandler(w, r, http.StatusInternalServerError)
		return
	}

	redirect_url := config.RedirectURL

	if len(r.Form["url"][0]) != 0 {
		redirect_url = r.Form["url"][0]
	}

	log.WithFields(log.Fields{
		"url": redirect_url,
	}).Debug("Redirecting user")

	vars := Page{Title: config.Title, URL: redirect_url}

	err = templates.ExecuteTemplate(w, "thankYouPage", vars)
	if err != nil {
		log.Error(err.Error())
		errorHandler(w, r, http.StatusInternalServerError)
		return
	}

	err = dbSave(r.Form["email"][0], r.Form["id"][0], r.Form["ap"][0], r.Form["ssid"][0])
	if err != nil {
		log.Error(err.Error())
	}
}

func errorHandler(w http.ResponseWriter, r *http.Request, code int) {
	w.WriteHeader(code)
	vars := Page{Title: config.Title}
	err := templates.ExecuteTemplate(w, "errorPage", vars)
	if err != nil {
		log.Error(err.Error())
	}
}

func loadConfiguration(file string) (*Config, error) {
	yamlFile, err := ioutil.ReadFile(*configFile)
	if err != nil {
		return nil, err
	}

	config := new(Config)
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.WithFields(log.Fields{
			"remote_addr": r.RemoteAddr,
			"method":      r.Method,
			"url":         r.URL,
		}).Debug("Request")
		handler.ServeHTTP(w, r)
	})
}

func info() string {
	return fmt.Sprintf("version=%s, commit=%s, branch=%s", Version, Commit, Branch)
}

func main() {
	flag.Parse()

	if *version {
		fmt.Println(info())
		os.Exit(0)
	}

	if *verbose {
		log.SetLevel(log.DebugLevel)
	}

	log.Info("Starting up...")
	log.Debug(info())

	var err error
	config, err = loadConfiguration(*configFile)
	if err != nil {
		log.Fatalf("Configuration error, aborting: %s", err)
	}

	fs := http.FileServer(http.Dir(*assetDir))
	http.Handle("/assets/", http.StripPrefix("/assets/", fs))
	http.HandleFunc("/guest/s/default/", landingHandler)
	http.HandleFunc("/form", formHandler)
	log.Fatal(http.ListenAndServe(*listenAddress, logRequest(http.DefaultServeMux)))

}
