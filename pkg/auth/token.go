package auth

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/mchmarny/dctl/pkg/net"
	"github.com/pkg/errors"
)

const (
	deviceCodeURL = "https://github.com/login/device/code"
	accessCodeURL = "https://github.com/login/oauth/access_token"
	deviceScopes  = "" // public_repo read:repo read:org read:user read:org_hook read:public_key read:gpg_key read:notifications read:gist write:gpg_key write:notifications write:public_key write:gist write:repo write:org write:user write:org_hook
	grantType     = "urn:ietf:params:oauth:grant-type:device_code"
)

type DeviceCode struct {
	// The device verification code is 40 characters and used to verify the device.
	DeviceCode string `json:"device_code,omitempty"`
	// The user verification code is displayed on the device so the user
	// can enter the code in a browser. This code is 8 characters with a
	// hyphen in the middle.
	UserCode string `json:"user_code,omitempty"`
	// The verification URL where users need to enter the user_code
	VerificationURL string `json:"verification_uri,omitempty"`
	// The number of seconds before the device_code and user_code expire.
	// The default is 900 seconds or 15 minutes.
	ExpiresInSec int `json:"expires_in,omitempty"`
	// The minimum number of seconds that must pass before you can make a new access token request
	// (POST https://github.com/login/oauth/access_token) to complete the device authorization.
	// For example, if the interval is 5, then you cannot make a new request until 5 seconds pass.
	// If you make more than one request over 5 seconds, then you will hit the rate limit and receive a slow_down error.
	Interval int `json:"interval,omitempty"`
}

type AccessTokenResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

func GetDeviceCode(clientID string) (*DeviceCode, error) {
	if clientID == "" {
		return nil, errors.New("clientID is required")
	}

	req, err := http.NewRequest(http.MethodPost, deviceCodeURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	q := req.URL.Query()
	q.Add("client_id", clientID)
	q.Add("scope", deviceScopes)
	req.URL.RawQuery = q.Encode()

	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")

	client, err := net.GetHTTPClient()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get http client")
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body := ""
		if b, err := ioutil.ReadAll(res.Body); err == nil {
			body = string(b)
		}

		return nil, errors.Errorf("failed to get device code: %s - %s - %s", res.Status, req.URL, body)
	}

	var dc DeviceCode
	if err := json.NewDecoder(res.Body).Decode(&dc); err != nil {
		return nil, errors.Wrap(err, "failed to decode response")
	}

	return &dc, nil
}

// TODO: decode the possible error codes
//       https://docs.github.com/en/developers/apps/building-oauth-apps/authorizing-oauth-apps#error-codes-for-the-device-flow

func GetToken(clientID string, code *DeviceCode) (*AccessTokenResponse, error) {
	if clientID == "" {
		return nil, errors.New("clientID is required")
	}

	if code == nil {
		return nil, errors.New("device code is nil")
	}

	expiresAt := time.Now().UTC().Add(time.Duration(code.ExpiresInSec) * time.Second)

	req, err := http.NewRequest(http.MethodPost, accessCodeURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	q := req.URL.Query()
	q.Add("client_id", clientID)
	q.Add("device_code", code.DeviceCode) // device verification code from the POST request
	q.Add("grant_type", grantType)
	req.URL.RawQuery = q.Encode()

	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")

	client, err := net.GetHTTPClient()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get http client")
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request")
	}

	defer res.Body.Close()

	var t AccessTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&t); err != nil {
		return nil, errors.Wrap(err, "failed to decode response")
	}

	if time.Now().UTC().After(expiresAt) {
		return nil, errors.New("access token expired")
	}

	if t.AccessToken == "" {
		return nil, errors.New("access token is empty")
	}

	return &t, nil
}
