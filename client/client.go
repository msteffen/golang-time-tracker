package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// String renders an Interval as a string (left out of api.go so that that file
// is only types and has no imports
func (i Interval) String() string {
	duration := time.Duration(i.End-i.Start) * time.Second
	start := time.Unix(i.Start, 0)
	return fmt.Sprintf("[%s starting %s (%s)]", duration, start, i.Label)
}

// HTTPError represents an error returned by an HTTP service
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("(%d/%s) %s", e.StatusCode, http.StatusText(e.StatusCode), e.Message)
}

// Client is a an HTTP client wrapper, with convenience functions for Get and
// Post requests sent to paths under a single destination. It also wraps non-200
// http responses in an error.
type Client struct {
	Address string
}

// url converts the API endpoint 'address' into a pseudo-URL that the golang
// http library can use. Note that the first path component after the protocol
// is parsed as the domain, which is a mandatory component of a URL but is also
// ignored when communicating over a unix socket, so we provide a standard
// throwaway domain of "socket"
func (c *Client) url(path string) string {
	return "http://" + c.Address + "/" + strings.TrimPrefix(path, "/")
}

func httpRespToError(resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return resp, err
	}
	if resp.StatusCode != http.StatusOK {
		msgBytes, err := ioutil.ReadAll(resp.Body)
		msg := string(bytes.TrimSpace(msgBytes))
		if err != nil {
			msg = fmt.Sprintf("time-tracker could not read response body: %v", err)
		}
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}
	return resp, err
}

// Get is a convenience function for Get requests, that sends all such
// requests to the client's socket path/URL.
func (c *Client) Get(path string) (*http.Response, error) {
	return httpRespToError(http.DefaultClient.Get(c.url(path)))
}

// PostString is a convenience function for Post requests, that sends all such
// requests to the client's socket path/URL.
func (c *Client) PostString(address string, body string) (*http.Response, error) {
	r := strings.NewReader(body)
	return httpRespToError(
		http.DefaultClient.Post(c.url(address), "application/json", r))
}

// Post is a convenience function for Post requests, that sends all such
// requests to the client's socket path/URL.
func (c *Client) Post(address string, body io.Reader) (*http.Response, error) {
	return httpRespToError(
		http.DefaultClient.Post(c.url(address), "application/json", body))
}

func (c *Client) Status() (time.Duration, error) {
	resp, err := c.Get("/status")
	if err != nil {
		return 0, err
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return 0, err
	}
	dur, err := time.ParseDuration(buf.String())
	if err != nil {
		return dur, fmt.Errorf("could not parse duration from /status: %v", err)
	}
	return dur, nil
}

// Tick is a convenience function that POSTs to the /tick URL endpoint
func (c *Client) Tick(label string) (int64, error) {
	var resp *http.Response
	var err error
	if label == "" {
		resp, err = c.Get("/tick")
	} else {
		buf := bytes.Buffer{}
		json.NewEncoder(&buf).Encode(TickRequest{Label: label})
		resp, err = c.Post("/tick", &buf)
	}
	if err != nil {
		return 0, err
	}
	var tickResp TickResponse
	if err := json.NewDecoder(resp.Body).Decode(&tickResp); err != nil {
		return 0, fmt.Errorf("error decoding response: %v", err)
	}
	return tickResp.Now, nil
}

// GetIntervals is a convenience function that wraps the /intervals URL endpoint
func (c *Client) GetIntervals(start, end time.Time) (*GetIntervalsResponse, error) {
	resp, err := c.Get(
		fmt.Sprintf("/intervals?start=%d&end=%d", start.Unix(), end.Unix()))
	if err != nil {
		return nil, err
	}

	var intervals GetIntervalsResponse
	if err := json.NewDecoder(resp.Body).Decode(&intervals); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}
	return &intervals, nil
}

func (c *Client) Watch(dir, label string) error {
	buf := bytes.Buffer{}
	json.NewEncoder(&buf).Encode(WatchRequest{Dir: dir, Label: label})
	_, err := c.Post("/watch", &buf)
	return err
}

func (c *Client) GetWatches() (*GetWatchesResponse, error) {
	resp, err := c.Get("/watches")
	if err != nil {
		return nil, err
	}

	var watches GetWatchesResponse
	if err := json.NewDecoder(resp.Body).Decode(&watches); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}
	return &watches, nil
}

func (c *Client) Clear() (retErr error) {
	_, err := c.PostString("/clear", `{"confirm":"yes"}`)
	return err
}
