package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/schema"
)

const (
	DefaultHTTPTimeout = 30 * time.Second
	DefaultAuthHeader  = "Api-Key"
)

type GraphqlQuery struct {
	Query     string      `json:"query"`
	Variables interface{} `json:"variables"` // map[string]interface really...
}

type Endpoint struct {
	URL  string
	Auth AuthConfig
	HTTP HTTPConfig
}

type AuthConfig struct {
	Disable bool
	Header  string
	APIKey  string
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

// Fetch returns everything we know how to get about the schema of
// an endpoint
func (e *Endpoint) Fetch() (*schema.Schema, error) {
	s, err := e.FetchSchema()
	if err != nil {
		return nil, err
	}

	// Grab the root mutation name and fetch that type
	if s.MutationType != nil {
		m, mErr := e.FetchType(s.MutationType.Name)
		if mErr != nil {
			return nil, mErr
		}
		s.MutationType = m
	}

	if s.QueryType != nil {
		m, mErr := e.FetchType(s.QueryType.Name)
		if mErr != nil {
			return nil, mErr
		}
		s.QueryType = m
	}

	// Fetch all of the other data types
	t, err := e.FetchSchemaTypes()
	if err != nil {
		return nil, err
	}
	s.Types = t

	return s, nil
}

// FetchSchema returns basic info about the schema
func (e *Endpoint) FetchSchema() (*schema.Schema, error) {
	log.Infof("fetching schema from endpoint: %s", e.URL)
	query := GraphqlQuery{
		Query: schema.QuerySchema,
	}

	resp, err := e.fetch(query)
	if err != nil {
		return nil, err
	}

	return &resp.Data.Schema, nil
}

// FetchTypes gathers all of the data types in the schema
func (e *Endpoint) FetchSchemaTypes() ([]*schema.Type, error) {
	log.Info("fetching schema types")
	query := GraphqlQuery{
		Query: schema.QuerySchemaTypes,
	}

	resp, err := e.fetch(query)
	if err != nil {
		return nil, err
	}

	return resp.Data.Schema.Types, nil
}

func (e *Endpoint) FetchType(name string) (*schema.Type, error) {
	if name == "" {
		return nil, errors.New("can not fetch type without a name")
	}

	log.WithFields(log.Fields{
		"type": name,
	}).Info("fetching type")

	query := GraphqlQuery{
		Query: schema.QueryType,
		Variables: schema.QueryTypeVars{
			Name: name,
		},
	}

	resp, err := e.fetch(query)
	if err != nil {
		return nil, err
	}

	return &resp.Data.Type, nil
}

// fetch does the heavy lifting to return the schema data
func (e *Endpoint) fetch(query GraphqlQuery) (*schema.QueryResponse, error) {
	if e.URL == "" {
		return nil, errors.New("unable to fetch from empty URL")
	}

	j, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	log.WithFields(log.Fields{
		"query": string(j),
	}).Trace("using query")
	reqBody := bytes.NewBuffer(j)

	req, err := http.NewRequestWithContext(context.Background(), "POST", e.URL, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if !e.Auth.Disable {
		if e.Auth.APIKey != "" {
			log.WithFields(log.Fields{
				"header": req.Header,
			}).Trace("setting API Key header")
			req.Header.Set(e.Auth.Header, e.Auth.APIKey)
		}
	}

	log.Trace("creating HTTP client")
	tr := &http.Transport{
		MaxIdleConns:          10,
		IdleConnTimeout:       e.HTTP.Timeout,
		ResponseHeaderTimeout: e.HTTP.Timeout,
	}

	client := &http.Client{Transport: tr}

	log.WithFields(log.Fields{
		"header":   e.Auth.Header,
		"endpoint": e.URL,
		"method":   req.Method,
	}).Debug("sending request")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	log.WithFields(log.Fields{
		"status_code": resp.StatusCode,
	}).Debug("request completed")
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	return schema.ParseResponse(resp)
}
