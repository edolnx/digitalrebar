package api

// Apache 2 License 2015 by Rob Hirschfeld for RackN portions of
// source based on
// https://code.google.com/p/mlab-ns2/source/browse/gae/ns/digest/digest.go
import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path"

	"github.com/VictorLowther/jsonpatch"
	"github.com/digitalrebar/digitalrebar/go/common/cert"
	"github.com/digitalrebar/digitalrebar/go/rebar-api/datatypes"
)

type challenge interface {
	parseChallenge(resp *http.Response) error
	authorize(method, uri string, req *http.Request) error
}

// Client wraps http.Client to add our auth primitives.
type Client struct {
	*http.Client
	Challenge challenge
	URL       string
}

// The Rebar API is exposed over a digest authenticated HTTP(s)
// connection.  This file implements all of the basic REST and HTTP
// operations that Rebar uses.

// TrustedSession builds a Client that can only operate inside the local trust zone.
// It assumes that there is a local Consul server that it can use to look up the
// trust-me service and the internal endpoint for the Rebar API.
func TrustedSession(URL, username string) (*Client, error) {
	c, err := cert.Client("internal", "rebar-client")
	if err != nil {
		return nil, err
	}
	res := &Client{URL: URL, Client: c, Challenge: challengeTrusted(username)}
	user := &User{}
	if err := res.Fetch(user, username); err != nil {
		return nil, fmt.Errorf("Unable to verify existence of %s user, cannot use trusted session: %v", username, err)
	}
	return res, nil
}

// Session establishes a new connection to Rebar.  You must call
// this function before using any other functions in the rebar
// package.  Session stores its information in a private global variable.
func Session(URL, User, Password string) (*Client, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	c := &Client{
		URL:    URL,
		Client: &http.Client{Transport: tr},
	}
	// retrieve the digest info from the 301 message
	resp, e := c.Head(c.URL + path.Join(datatypes.API_PATH, "digest"))
	if e != nil {
		return nil, e
	}
	if resp.StatusCode != 401 && resp.StatusCode != 200 {
		return nil, fmt.Errorf("Expected Digest Challenge os SAML Redirect Missing on URL %s got %s", URL, resp.Status)
	}

	// We may be SAML Auth
	if resp.StatusCode == 200 {
		c.Challenge = &challengeSAML{
			Username: User,
			Password: Password,
		}
	} else {
		c.Challenge = &challengeDigest{
			Username: User,
			Password: Password,
		}
	}

	err := c.Challenge.parseChallenge(resp)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) basicRequest(method, uri string, objIn []byte) (resp *http.Response, err error) {
	var body io.Reader

	if objIn != nil {
		body = bytes.NewReader(objIn)
	}
	log.Printf("uri: %s, url: %s", uri, c.URL+uri)
	// Construct the http.Request.
	req, err := http.NewRequest(method, c.URL+uri, body)
	if err != nil {
		return nil, err
	}
	err = c.Challenge.authorize(method, uri, req)
	if err != nil {
		return nil, err
	}
	if method == "PATCH" {
		req.Header.Set("Content-Type", jsonpatch.ContentType)
	} else {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "gobar/v1.0")
	req.Header.Set("Accept", "application/json")
	resp, err = c.Do(req)
	if err == nil {
		return
	}
	if resp != nil {
		resp.Body.Close()
	}
	return nil, err
}

// Request makes a general call to the Rebar API.
// method is the raw HTTP method to use
// uri is the section of the API to call.
// objIn is the raw data to be passed in the request body
// objOut is the raw request body (if any)
// err is the error of any occurred.
func (c *Client) request(method, uri string, objIn []byte) (objOut []byte, err error) {
	resp, err := c.basicRequest(method, uri, objIn)
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode == 401 {
		err = c.Challenge.parseChallenge(resp)
		if err != nil {
			return nil, err
		}
		resp, err = c.basicRequest(method, uri, objIn)
		if err != nil {
			return nil, err
		}
		if resp.Body != nil {
			defer resp.Body.Close()
		}
	}
	objOut, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Expected status in the 200 range, got %s", resp.Status)
	}
	return objOut, nil
}

// list is a helper specialized to get lists of objects.
func (c *Client) list(res interface{}, uri ...string) (err error) {
	buf, err := c.request("GET", path.Join(uri...), nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf, &res)
}

func (c *Client) match(vals map[string]interface{}, res interface{}, uri ...string) (err error) {
	inbuf, err := json.Marshal(vals)
	buf, err := c.request("POST",
		path.Join(path.Join(uri...), "match"),
		inbuf)
	if err != nil {
		return nil
	}
	return json.Unmarshal(buf, &res)
}
