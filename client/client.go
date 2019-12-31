package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
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
	*http.Client
	path string
}

// url converts the API endpoint 'dest' into a pseudo-URL that the golang http
// library can use. Note that the first path component after the protocol is
// parsed as the domain, which is a mandatory component of a URL but is also
// ignored when communicating over a unix socket, so we provide a standard
// throwaway domain of "socket"
func (c *Client) url(dest string) string {
	return "http://socket/" + strings.TrimPrefix(dest, "/")
}

func httpRespToError(resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return resp, err
	}
	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    string(bytes.TrimSpace(buf.Bytes())),
		}
	}
	return resp, err
}

// Get is a convenience function for Get requests, that sends all such
// requests to the client's socket path/URL.
func (c *Client) Get(dest string) (*http.Response, error) {
	return httpRespToError(c.Client.Get(c.url(dest)))
}

// PostString is a convenience function for Post requests, that sends all such
// requests to the client's socket path/URL.
func (c *Client) PostString(dest string, body string) (*http.Response, error) {
	return httpRespToError(c.Client.Post(c.url(dest), "application/json", strings.NewReader(body)))
}

// Post is a convenience function for Post requests, that sends all such
// requests to the client's socket path/URL.
func (c *Client) Post(dest string, body io.Reader) (*http.Response, error) {
	return httpRespToError(c.Client.Post(c.url(dest), "application/json", body))
}

// GetClient initializes a new client that sends all requests to the socket/URL
// at 'path'.
func GetClient(path string) *Client {
	// Create the HTTP client and return it
	return &Client{
		Client: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					dialer := &net.Dialer{Timeout: 5 * time.Second}
					return dialer.DialContext(ctx, "unix", path)
				},
			},
		},
		path: path,
	}
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

// Tick is a convenience function that wraps the /tick URL endpoint
func (c *Client) Tick(label string) error {
	buf := bytes.Buffer{}
	json.NewEncoder(&buf).Encode(TickRequest{Label: label})
	_, err := c.Post("/tick", &buf)
	return err
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
