package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn_go/client/call"
	"github.com/fnproject/fn_go/client/operations"
)

type ErrMsg struct {
	Message string `json:"message"`
}

type TimeoutBody struct {
	Error  ErrMsg `json:"error"`
	CallID string `json:"request_id"`
}

func CallAsync(t *testing.T, u url.URL, content io.Reader) string {
	output := &bytes.Buffer{}
	_, err := CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}

	expectedOutput := "call_id"
	if !strings.Contains(output.String(), expectedOutput) {
		t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}

	type CallID struct {
		CallID string `json:"call_id"`
	}

	callID := &CallID{}
	json.NewDecoder(output).Decode(callID)

	if callID.CallID == "" {
		t.Errorf("`call_id` not suppose to be empty string")
	}
	t.Logf("Async execution call ID: %v", callID.CallID)
	return callID.CallID
}

func TestRouteExecutions(t *testing.T) {
	newRouteType := "async"

	t.Run("run-sync-fnproject/hello-no-input", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, "sync",
			s.Format, s.RouteConfig, s.RouteHeaders)

		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

		content := &bytes.Buffer{}
		output := &bytes.Buffer{}
		_, err := CallFN(u.String(), content, output, "POST", []string{})
		if err != nil {
			t.Errorf("Got unexpected error: %v", err)
		}
		expectedOutput := "Hello World!\n"
		if !strings.Contains(expectedOutput, output.String()) {
			t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("run-sync-fnproject/hello-with-input", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, "sync",
			s.Format, s.RouteConfig, s.RouteHeaders)

		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

		content := &bytes.Buffer{}
		json.NewEncoder(content).Encode(struct {
			Name string
		}{Name: "John"})
		output := &bytes.Buffer{}
		_, err := CallFN(u.String(), content, output, "POST", []string{})
		if err != nil {
			t.Errorf("Got unexpected error: %v", err)
		}
		expectedOutput := "Hello John!\n"
		if !strings.Contains(expectedOutput, output.String()) {
			t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)

	})

	t.Run("run-async-fnproject/hello", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, "sync",
			s.Format, s.RouteConfig, s.RouteHeaders)

		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

		_, err := UpdateRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, newRouteType, s.Format,
			s.Memory, s.RouteConfig, s.RouteHeaders, "")

		CheckRouteResponseError(t, err)

		CallAsync(t, u, &bytes.Buffer{})
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("run-async-fnproject/hello-with-status-check", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, "sync",
			s.Format, s.RouteConfig, s.RouteHeaders)

		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

		_, err := UpdateRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, newRouteType, s.Format,
			s.Memory, s.RouteConfig, s.RouteHeaders, "")

		CheckRouteResponseError(t, err)

		callID := CallAsync(t, u, &bytes.Buffer{})
		time.Sleep(time.Second * 10)
		cfg := &call.GetAppsAppCallsCallParams{
			Call:    callID,
			App:     s.AppName,
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		callResponse, err := s.Client.Call.GetAppsAppCallsCall(cfg)
		if err != nil {
			switch err.(type) {
			case *call.GetAppsAppCallsCallNotFound:
				msg := err.(*call.GetAppsAppCallsCallNotFound).Payload.Error.Message
				t.Errorf("Unexpected error occurred: %v.", msg)
			}
		}
		callObject := callResponse.Payload.Call

		if callObject.AppName != s.AppName {
			t.Errorf("Call object app name mismatch.\n\tExpected: %v\n\tActual:%v", s.AppName, callObject.AppName)
		}
		if callObject.ID != callID {
			t.Errorf("Call object ID mismatch.\n\tExpected: %v\n\tActual:%v", callID, callObject.ID)
		}
		if callObject.Path != s.RoutePath {
			t.Errorf("Call object route path mismatch.\n\tExpected: %v\n\tActual:%v", s.RoutePath, callObject.Path)
		}
		if callObject.Status != "success" {
			t.Errorf("Call object status mismatch.\n\tExpected: %v\n\tActual:%v", "success", callObject.Status)
		}

		DeleteApp(t, s.Context, s.Client, s.AppName)

	})

	t.Run("exec-timeout-test", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		routePath := "/" + RandStringBytes(10)
		image := "funcy/timeout:0.0.1"
		routeType := "sync"

		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, routePath, image, routeType,
			s.Format, s.RouteConfig, s.RouteHeaders)

		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, routePath)

		content := &bytes.Buffer{}
		json.NewEncoder(content).Encode(struct {
			Seconds int64 `json:"seconds"`
		}{Seconds: 31})
		output := &bytes.Buffer{}

		headers, _ := CallFN(u.String(), content, output, "POST", []string{})

		if !strings.Contains(output.String(), "Timed out") {
			t.Errorf("Must fail because of timeout, but got error message: %v", output.String())
		}

		cfg := &call.GetAppsAppCallsCallParams{
			Call:    headers.Get("FN_CALL_ID"),
			App:     s.AppName,
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		callObj, err := s.Client.Call.GetAppsAppCallsCall(cfg)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if !strings.Contains("timeout", callObj.Payload.Call.Status) {
			t.Errorf("Call status mismatch.\n\tExpected: %v\n\tActual: %v",
				"output", "callObj.Payload.Call.Status")
		}

		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("exec-multi-log-test", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		routePath := "/multi-log"
		image := "funcy/multi-log:0.0.1"
		routeType := "async"

		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, routePath, image, routeType,
			s.Format, s.RouteConfig, s.RouteHeaders)

		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, routePath)

		callID := CallAsync(t, u, &bytes.Buffer{})
		time.Sleep(15 * time.Second)

		cfg := &operations.GetAppsAppCallsCallLogParams{
			Call:    callID,
			App:     s.AppName,
			Context: s.Context,
		}

		logObj, err := s.Client.Operations.GetAppsAppCallsCallLog(cfg)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if logObj.Payload.Log.Log == "" {
			t.Errorf("Log entry must not be empty!")
		}
		if !strings.Contains(logObj.Payload.Log.Log, "First line") {
			t.Errorf("Log entry must contain `First line` "+
				"string, but got: %v", logObj.Payload.Log.Log)
		}
		if !strings.Contains(logObj.Payload.Log.Log, "Second line") {
			t.Errorf("Log entry must contain `Second line` "+
				"string, but got: %v", logObj.Payload.Log.Log)
		}

		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("verify-headers-separator", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		routePath := "/os.environ"
		image := "denismakogon/os.environ"
		routeType := "sync"
		CreateRoute(t, s.Context, s.Client, s.AppName, routePath, image, routeType,
			s.Format, s.RouteConfig, s.RouteHeaders)

		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, routePath)
		content := &bytes.Buffer{}
		output := &bytes.Buffer{}
		CallFN(u.String(), content, output, "POST",
			[]string{
				"ACCEPT: application/xml",
				"ACCEPT: application/json; q=0.2",
			})
		res := output.String()
		if !strings.Contains("application/xml, application/json; q=0.2", res) {
			t.Errorf("HEADER_ACCEPT='application/xml, application/json; q=0.2' "+
				"should be in output, have:%s\n", res)
		}
		DeleteRoute(t, s.Context, s.Client, s.AppName, routePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("exec-log-test", func(t *testing.T) {
		//XXX: Fix this test.
		t.Skip("Flaky test needs to be rewritten. https://github.com/fnproject/fn/issues/253")
		t.Parallel()
		s := SetupDefaultSuite()
		routePath := "/log"
		image := "funcy/log:0.0.1"
		routeType := "async"

		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, routePath, image, routeType,
			s.Format, s.RouteConfig, s.RouteHeaders)

		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, routePath)
		content := &bytes.Buffer{}
		json.NewEncoder(content).Encode(struct {
			Size int
		}{Size: 20})

		callID := CallAsync(t, u, content)
		time.Sleep(10 * time.Second)

		cfg := &operations.GetAppsAppCallsCallLogParams{
			Call:    callID,
			App:     s.AppName,
			Context: s.Context,
		}

		_, err := s.Client.Operations.GetAppsAppCallsCallLog(cfg)

		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}

		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("exec-oversized-log-test", func(t *testing.T) {
		t.Parallel()
		t.Skip("Skipped until fix for https://gitlab-odx.oracle.com/odx/functions/issues/86.")

		s := SetupDefaultSuite()
		routePath := "/log"
		image := "funcy/log:0.0.1"
		routeType := "async"

		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, routePath, image, routeType,
			s.Format, s.RouteConfig, s.RouteHeaders)

		size := 1 * 1024 * 1024 * 1024
		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, routePath)
		content := &bytes.Buffer{}
		json.NewEncoder(content).Encode(struct {
			Size int
		}{Size: size}) //exceeding log by 1 symbol

		callID := CallAsync(t, u, content)
		time.Sleep(5 * time.Second)

		cfg := &operations.GetAppsAppCallsCallLogParams{
			Call:    callID,
			App:     s.AppName,
			Context: s.Context,
		}

		logObj, err := s.Client.Operations.GetAppsAppCallsCallLog(cfg)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if len(logObj.Payload.Log.Log) >= size {
			t.Errorf("Log entry suppose to be truncated up to expected size %v, got %v",
				size/1024, len(logObj.Payload.Log.Log))
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

}
