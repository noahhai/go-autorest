package azure

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/mocks"
)

const (
	TestResource = "SomeResource"
	TestClientID = "SomeClientID"
	TestTenantID = "SomeTenantID"
)

const MockDeviceCodeResponse = `
{
	"device_code": "10000-40-1234567890",
	"user_code": "ABCDEF",
	"verification_url": "http://aka.ms/deviceauth",
	"expires_in": "900",
	"interval": "0"
}
`

const MockDeviceTokenResponse = `{
	"access_token": "accessToken",
	"refresh_token": "refreshToken",
	"expires_in": "1000",
	"expires_on": "2000",
	"not_before": "3000",
	"resource": "resource",
	"token_type": "type"
}
`

func TestDeviceCodeIncludesResource(t *testing.T) {
	sender := mocks.NewSender()
	sender.EmitContent(MockDeviceCodeResponse)
	sender.EmitStatus("OK", 200)
	client := &autorest.Client{Sender: sender}

	code, err := InitiateDeviceAuth(client, TestClientID, TestTenantID, TestResource)
	if err != nil {
		t.Errorf("azure: unexpected error initiating device auth")
	}

	if code.Resource != TestResource {
		t.Errorf("azure: InitiateDeviceAuth failed to stash the resource in the DeviceCode struct")
	}
}

func TestDeviceCodeReturnsErrorIfSendingFails(t *testing.T) {
	sender := mocks.NewSender()
	sender.EmitErrors(1)
	sender.SetError(fmt.Errorf("this is an error"))
	client := &autorest.Client{Sender: sender}

	_, err := InitiateDeviceAuth(client, TestClientID, TestTenantID, TestResource)
	if err == nil || !strings.Contains(err.Error(), errCodeSendingFails) {
		t.Errorf("azure: failed to get correct error expected(%s) actual(%s)", errCodeSendingFails, err.Error())
	}
}

func TestDeviceCodeReturnsErrorIfBadRequest(t *testing.T) {
	sender := mocks.NewSender()
	body := mocks.NewBody("doesn't matter")
	sender.SetResponse(mocks.NewResponseWithBodyAndStatus(body, 400, "Bad Request"))
	client := &autorest.Client{Sender: sender}

	_, err := InitiateDeviceAuth(client, TestClientID, TestTenantID, TestResource)
	if err == nil || !strings.Contains(err.Error(), errCodeHandlingFails) {
		t.Errorf("azure: failed to get correct error expected(%s) actual(%s)", errCodeHandlingFails, err.Error())
	}

	if body.IsOpen() {
		t.Errorf("response body was left open!")
	}
}

func TestDeviceCodeReturnsErrorIfCannotDeserializeDeviceCode(t *testing.T) {
	gibberishJSON := strings.Replace(MockDeviceCodeResponse, "expires_in", "\":, :gibberish", -1)
	sender := mocks.NewSender()
	body := mocks.NewBody(gibberishJSON)
	sender.SetResponse(mocks.NewResponseWithBodyAndStatus(body, 200, "OK"))
	client := &autorest.Client{Sender: sender}

	_, err := InitiateDeviceAuth(client, TestClientID, TestTenantID, TestResource)
	if err == nil || !strings.Contains(err.Error(), errCodeHandlingFails) {
		t.Errorf("azure: failed to get correct error expected(%s) actual(%s)", errCodeHandlingFails, err.Error())
	}

	if body.IsOpen() {
		t.Errorf("response body was left open!")
	}
}

func deviceCode() *DeviceCode {
	var deviceCode DeviceCode
	json.Unmarshal([]byte(MockDeviceCodeResponse), &deviceCode)
	deviceCode.Resource = TestResource
	deviceCode.ClientID = TestClientID
	deviceCode.TenantID = TestTenantID
	return &deviceCode
}

func TestDeviceTokenReturns(t *testing.T) {
	sender := mocks.NewSender()
	body := mocks.NewBody(MockDeviceTokenResponse)
	sender.SetResponse(mocks.NewResponseWithBodyAndStatus(body, 200, "OK"))
	client := &autorest.Client{Sender: sender}

	_, err := WaitForUserCompletion(client, deviceCode())
	if err != nil {
		t.Errorf("azure: got error unexpectedly")
	}

	if body.IsOpen() {
		t.Errorf("response body was left open!")
	}
}

func TestDeviceTokenReturnsErrorIfSendingFails(t *testing.T) {
	sender := mocks.NewSender()
	sender.EmitErrors(1)
	sender.SetError(fmt.Errorf("this is an error"))
	client := &autorest.Client{Sender: sender}

	_, err := WaitForUserCompletion(client, deviceCode())
	if err == nil || !strings.Contains(err.Error(), errTokenSendingFails) {
		t.Errorf("azure: failed to get correct error expected(%s) actual(%s)", errTokenSendingFails, err.Error())
	}
}

func TestDeviceTokenReturnsErrorIfServerError(t *testing.T) {
	sender := mocks.NewSender()
	body := mocks.NewBody("")
	sender.SetResponse(mocks.NewResponseWithBodyAndStatus(body, 500, "Internal Server Error"))
	client := &autorest.Client{Sender: sender}

	_, err := WaitForUserCompletion(client, deviceCode())
	if err == nil || !strings.Contains(err.Error(), errTokenHandlingFails) {
		t.Errorf("azure: failed to get correct error expected(%s) actual(%s)", errTokenHandlingFails, err.Error())
	}

	if body.IsOpen() {
		t.Errorf("response body was left open!")
	}
}

func TestDeviceTokenReturnsErrorIfCannotDeserializeDeviceToken(t *testing.T) {
	gibberishJSON := strings.Replace(MockDeviceTokenResponse, "expires_in", ";:\"gibberish", -1)
	sender := mocks.NewSender()
	body := mocks.NewBody(gibberishJSON)
	sender.SetResponse(mocks.NewResponseWithBodyAndStatus(body, 200, "OK"))
	client := &autorest.Client{Sender: sender}

	_, err := WaitForUserCompletion(client, deviceCode())
	if err == nil || !strings.Contains(err.Error(), errTokenHandlingFails) {
		t.Errorf("azure: failed to get correct error expected(%s) actual(%s)", errTokenHandlingFails, err.Error())
	}

	if body.IsOpen() {
		t.Errorf("response body was left open!")
	}
}

func errorDeviceTokenResponse(message string) string {
	return `{ "error": "` + message + `" }`
}

func TestDeviceTokenReturnsErrorIfAuthorizationPending(t *testing.T) {
	sender := mocks.NewSender()
	body := mocks.NewBody(errorDeviceTokenResponse("authorization_pending"))
	sender.SetResponse(mocks.NewResponseWithBodyAndStatus(body, 400, "Bad Request"))
	client := &autorest.Client{Sender: sender}

	_, err := CheckForUserCompletion(client, deviceCode())
	if err != ErrDeviceAuthorizationPending {
		t.Errorf("!!!")
	}

	if body.IsOpen() {
		t.Errorf("response body was left open!")
	}
}

func TestDeviceTokenReturnsErrorIfSlowDown(t *testing.T) {
	sender := mocks.NewSender()
	body := mocks.NewBody(errorDeviceTokenResponse("slow_down"))
	sender.SetResponse(mocks.NewResponseWithBodyAndStatus(body, 400, "Bad Request"))
	client := &autorest.Client{Sender: sender}

	_, err := CheckForUserCompletion(client, deviceCode())
	if err != ErrDeviceSlowDown {
		t.Errorf("!!!")
	}

	if body.IsOpen() {
		t.Errorf("response body was left open!")
	}
}

type deviceTokenSender struct {
	errorString string
	attempts    int
}

func newDeviceTokenSender(deviceErrorString string) *deviceTokenSender {
	return &deviceTokenSender{errorString: deviceErrorString, attempts: 0}
}

func (s *deviceTokenSender) Do(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	if s.attempts < 1 {
		s.attempts++
		resp = mocks.NewResponseWithContent(errorDeviceTokenResponse(s.errorString))
	} else {
		resp = mocks.NewResponseWithContent(MockDeviceTokenResponse)
	}
	return resp, nil
}

// since the above only exercise CheckForUserCompletion, we repeat the test here,
// but with the intent of showing that WaitForUserCompletion loops properly.
func TestDeviceTokenSucceedsWithIntermediateAuthPending(t *testing.T) {
	sender := newDeviceTokenSender("authorization_pending")
	client := &autorest.Client{Sender: sender}

	_, err := WaitForUserCompletion(client, deviceCode())
	if err != nil {
		t.Errorf("unexpected error occurred")
	}
}

// same as above but with SlowDown now
func TestDeviceTokenSucceedsWithIntermediateSlowDown(t *testing.T) {
	sender := newDeviceTokenSender("slow_down")
	client := &autorest.Client{Sender: sender}

	_, err := WaitForUserCompletion(client, deviceCode())
	if err != nil {
		t.Errorf("unexpected error occurred")
	}
}

func TestDeviceTokenReturnsErrorIfAccessDenied(t *testing.T) {
	sender := mocks.NewSender()
	body := mocks.NewBody(errorDeviceTokenResponse("access_denied"))
	sender.SetResponse(mocks.NewResponseWithBodyAndStatus(body, 400, "Bad Request"))
	client := &autorest.Client{Sender: sender}

	_, err := WaitForUserCompletion(client, deviceCode())
	if err != ErrDeviceAccessDenied {
		t.Errorf("azure: got wrong error expected(%s) actual(%s)", ErrDeviceAccessDenied.Error(), err.Error())
	}

	if body.IsOpen() {
		t.Errorf("response body was left open!")
	}
}

func TestDeviceTokenReturnsErrorIfCodeExpired(t *testing.T) {
	sender := mocks.NewSender()
	body := mocks.NewBody(errorDeviceTokenResponse("code_expired"))
	sender.SetResponse(mocks.NewResponseWithBodyAndStatus(body, 400, "Bad Request"))
	client := &autorest.Client{Sender: sender}

	_, err := WaitForUserCompletion(client, deviceCode())
	if err != ErrDeviceCodeExpired {
		t.Errorf("azure: got wrong error expected(%s) actual(%s)", ErrDeviceCodeExpired.Error(), err.Error())
	}

	if body.IsOpen() {
		t.Errorf("response body was left open!")
	}
}

func TestDeviceTokenReturnsErrorForUnknownError(t *testing.T) {
	sender := mocks.NewSender()
	body := mocks.NewBody(errorDeviceTokenResponse("unknown_error"))
	sender.SetResponse(mocks.NewResponseWithBodyAndStatus(body, 400, "Bad Request"))
	client := &autorest.Client{Sender: sender}

	_, err := WaitForUserCompletion(client, deviceCode())
	if err == nil {
		t.Errorf("failed to get error")
	}
	if err != ErrDeviceGeneric {
		t.Errorf("azure: got wrong error expected(%s) actual(%s)", ErrDeviceGeneric.Error(), err.Error())
	}

	if body.IsOpen() {
		t.Errorf("response body was left open!")
	}
}