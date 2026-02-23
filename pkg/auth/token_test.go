package auth

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDeviceCode_EmptyClientID(t *testing.T) {
	_, err := GetDeviceCode("", "repo")
	assert.Error(t, err)
}

func TestGetToken_EmptyClientID(t *testing.T) {
	_, err := GetToken("", &DeviceCode{})
	assert.Error(t, err)
}

func TestGetToken_NilCode(t *testing.T) {
	_, err := GetToken("test-client", nil)
	assert.Error(t, err)
}

func TestAccessTokenResponse_Unmarshal(t *testing.T) {
	raw := `{"access_token":"gho_test123","token_type":"bearer","scope":""}`
	var atr AccessTokenResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &atr))
	assert.Equal(t, "gho_test123", atr.AccessToken)
	assert.Equal(t, "bearer", atr.TokenType)
}

func TestDeviceCode_Unmarshal(t *testing.T) {
	raw := `{"device_code":"dc_test","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":900,"interval":5}`
	var dc DeviceCode
	require.NoError(t, json.Unmarshal([]byte(raw), &dc))
	assert.Equal(t, "dc_test", dc.DeviceCode)
	assert.Equal(t, "ABCD-1234", dc.UserCode)
	assert.Equal(t, "https://github.com/login/device", dc.VerificationURL)
	assert.Equal(t, 900, dc.ExpiresInSec)
	assert.Equal(t, 5, dc.Interval)
}
