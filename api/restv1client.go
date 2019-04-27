package api

// This file is^H^H used to be! autogenerated.

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// RestV1Client is a client for RestV1 servers.
type RestV1Client struct {
	base    string
	client  *http.Client
	retries int
}

// RestV1ClientError represents errors because of non-2xx HTTP response code.
type RestV1ClientError struct {
	code int
	msg  string
}

func newRestV1ClientError(code int) *RestV1ClientError {
	return &RestV1ClientError{
		code: code,
		msg:  fmt.Sprintf("server returned HTTP error code %d", code),
	}
}

// Code returns the HTTP response status code.
func (e *RestV1ClientError) Code() int {
	return e.code
}

// Error returns a human-readable error message.
func (e *RestV1ClientError) Error() string {
	return e.msg
}

// NewRestV1Client creates a new client to talk to the specified base URL
// and with the given timeout.
func NewRestV1Client(base string, timeout time.Duration, retries int) *RestV1Client {
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}

	return &RestV1Client{
		base: base,
		client: &http.Client{
			Timeout: timeout,
		},
		retries: retries,
	}
}

func (c *RestV1Client) callOnce(path string, req interface{}, resp interface{}) (retry, wait bool, err error) {

	// json-encode and gzip-compress the request body
	reqBody := &bytes.Buffer{}
	gzw := gzip.NewWriter(reqBody)
	if err = json.NewEncoder(gzw).Encode(req); err != nil {
		return
	}
	gzw.Close()

	// make HTTP request object
	hr, err := http.NewRequest("POST", c.base+path, reqBody)
	if err != nil {
		return
	}
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("Content-Encoding", "gzip")

	// perform HTTP request
	r, err := c.client.Do(hr)
	if err != nil {
		retry = true
		wait = !strings.Contains(strings.ToLower(err.Error()), "timeout")
		return
	}
	if r.StatusCode == 429 {
		err = errors.New("rate limited, retry after 60 seconds")
		return
	} else if r.StatusCode == 409 {
		err = errors.New("previous store for this server is still in progress")
		return
	} else if r.StatusCode/100 == 5 {
		err = newRestV1ClientError(r.StatusCode)
		retry = true
		wait = true
		return
	} else if r.StatusCode/100 != 2 {
		err = newRestV1ClientError(r.StatusCode)
		return
	}
	if r.Body == nil {
		err = fmt.Errorf("empty body received")
		return
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(body, resp)
	return
}

func (c *RestV1Client) call(path string, req interface{}, resp interface{}) error {
	var last error
	for i := 0; i < c.retries; i++ {
		retry, wait, err := c.callOnce(path, req, resp)
		last = err
		if err == nil {
			return nil
		}
		if !retry {
			return err
		}
		if wait {
			time.Sleep(c.client.Timeout)
		}
	}
	return last
}

// Quick calls RestV1.Quick
func (c *RestV1Client) Quick(req ReqQuick) (resp RespQuick, err error) {
	err = c.call("quick", req, &resp)
	return
}

// Report calls RestV1.Report
func (c *RestV1Client) Report(req ReqReport) (resp RespReport, err error) {
	err = c.call("report", req, &resp)
	return
}
