package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/schema"
)

const (
	DefaultHTTPTimeout = 30 * time.Second
	DefaultAuthHeader  = "Api-Key"
)

type Endpoint struct {
	URL  string
	Auth AuthConfig
	HTTP HTTPConfig
}

type AuthConfig struct {
	Header string
	APIKey string
}

type HTTPConfig struct {
	Timeout time.Duration
}

func NewEndpoint() *Endpoint {
	e := &Endpoint{
		Auth: AuthConfig{
			Header: DefaultAuthHeader,
		},
		HTTP: HTTPConfig{
			Timeout: DefaultHTTPTimeout,
		},
	}

	return e
}

func (e *Endpoint) Fetch() (*schema.Schema, error) {
	if e.URL == "" {
		return nil, errors.New("unable to fetch from empty URL")
	}

	query := struct {
		Query     string `json:"query"`
		Variables string `json:"variables"`
	}{
		Query: schema.Query,
	}

	j, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	reqBody := bytes.NewBuffer(j)

	log.Debug("creating request")

	req, err := http.NewRequestWithContext(context.Background(), "POST", e.URL, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if e.Auth.APIKey != "" {
		log.Debugf("setting API Key header: %s", e.Auth.Header)
		req.Header.Set(e.Auth.Header, e.Auth.APIKey)
	}

	log.Debug("making client")
	tr := &http.Transport{
		MaxIdleConns:          10,
		IdleConnTimeout:       e.HTTP.Timeout,
		ResponseHeaderTimeout: e.HTTP.Timeout,
	}
	client := &http.Client{Transport: tr}

	log.WithFields(log.Fields{
		"header": e.Auth.Header,
		"url":    e.URL,
		"apiKey": e.Auth.APIKey,
		"method": req.Method,
	}).Trace("sending request")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	log.WithFields(log.Fields{
		"status_code": resp.StatusCode,
	}).Debug("request completed")

	log.Debug("reading response body")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Trace(string(body))

	log.Debug("unmarshal JSON")
	ret := schema.QueryResponse{}
	err = json.Unmarshal(body, &ret)
	if err != nil {
		return nil, err
	}

	return &ret.Data.Schema, nil
}
