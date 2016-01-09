package http

import (
	"encoding/hex"
	"net/http"
	"reflect"
	"testing"

	"github.com/hashicorp/vault/vault"
)

func TestSysRootGenerationInit_Status(t *testing.T) {
	core, _, token := vault.TestCoreUnsealed(t)
	ln, addr := TestServer(t, core)
	defer ln.Close()
	TestServerAuth(t, addr, token)

	resp, err := http.Get(addr + "/v1/sys/root-generation/attempt")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	var actual map[string]interface{}
	expected := map[string]interface{}{
		"started":  false,
		"progress": float64(0),
		"required": float64(1),
		"complete": false,
	}
	testResponseStatus(t, resp, 200)
	testResponseBody(t, resp, &actual)
	expected["nonce"] = actual["nonce"]
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("\nexpected: %#v\nactual: %#v", expected, actual)
	}
}

func TestSysRootGenerationInit_Setup(t *testing.T) {
	core, _, token := vault.TestCoreUnsealed(t)
	ln, addr := TestServer(t, core)
	defer ln.Close()
	TestServerAuth(t, addr, token)

	resp := testHttpPut(t, token, addr+"/v1/sys/root-generation/attempt", nil)
	testResponseStatus(t, resp, 204)

	resp = testHttpGet(t, token, addr+"/v1/sys/root-generation/attempt")

	var actual map[string]interface{}
	expected := map[string]interface{}{
		"started":  true,
		"progress": float64(0),
		"required": float64(1),
		"complete": false,
	}
	testResponseStatus(t, resp, 200)
	testResponseBody(t, resp, &actual)
	expected["nonce"] = actual["nonce"]
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("\nexpected: %#v\nactual: %#v", expected, actual)
	}
}

func TestSysRootGenerationInit_Cancel(t *testing.T) {
	core, _, token := vault.TestCoreUnsealed(t)
	ln, addr := TestServer(t, core)
	defer ln.Close()
	TestServerAuth(t, addr, token)

	resp := testHttpPut(t, token, addr+"/v1/sys/root-generation/attempt", nil)
	testResponseStatus(t, resp, 204)

	resp = testHttpDelete(t, token, addr+"/v1/sys/root-generation/attempt")
	testResponseStatus(t, resp, 204)

	resp, err := http.Get(addr + "/v1/sys/root-generation/attempt")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	var actual map[string]interface{}
	expected := map[string]interface{}{
		"started":  false,
		"progress": float64(0),
		"required": float64(1),
		"complete": false,
	}
	testResponseStatus(t, resp, 200)
	testResponseBody(t, resp, &actual)
	expected["nonce"] = actual["nonce"]
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("\nexpected: %#v\nactual: %#v", expected, actual)
	}
}

func TestSysRootGeneration_badKey(t *testing.T) {
	core, _, token := vault.TestCoreUnsealed(t)
	ln, addr := TestServer(t, core)
	defer ln.Close()
	TestServerAuth(t, addr, token)

	resp := testHttpPut(t, token, addr+"/v1/sys/root-generation/update", map[string]interface{}{
		"key": "0123",
	})
	testResponseStatus(t, resp, 400)
}

func TestSysRootGeneration_Update(t *testing.T) {
	core, master, token := vault.TestCoreUnsealed(t)
	ln, addr := TestServer(t, core)
	defer ln.Close()
	TestServerAuth(t, addr, token)

	resp := testHttpPut(t, "asdf", addr+"/v1/sys/root-generation/attempt", nil)
	testResponseStatus(t, resp, 204)

	// We need to get the nonce first before we update
	resp, err := http.Get(addr + "/v1/sys/root-generation/attempt")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	var rootGenerationStatus map[string]interface{}
	testResponseStatus(t, resp, 200)
	testResponseBody(t, resp, &rootGenerationStatus)

	resp = testHttpPut(t, token, addr+"/v1/sys/root-generation/update", map[string]interface{}{
		"nonce": rootGenerationStatus["nonce"].(string),
		"key":   hex.EncodeToString(master),
	})

	var actual map[string]interface{}
	expected := map[string]interface{}{
		"complete": true,
		"nonce":    rootGenerationStatus["nonce"].(string),
		"progress": float64(1),
		"required": float64(1),
		"started":  true,
	}
	testResponseStatus(t, resp, 200)
	testResponseBody(t, resp, &actual)

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("\nexpected: %#v\nactual: %#v", expected, actual)
	}

	actual = map[string]interface{}{}
	expected = map[string]interface{}{
		"id":           "asdf",
		"display_name": "root",
		"meta":         interface{}(nil),
		"num_uses":     float64(0),
		"policies":     []interface{}{"root"},
		"orphan":       true,
		"ttl":          float64(0),
		"path":         "auth/token/root",
	}

	resp = testHttpGet(t, "asdf", addr+"/v1/auth/token/lookup-self")
	testResponseStatus(t, resp, 200)
	testResponseBody(t, resp, &actual)

	expected["creation_time"] = actual["data"].(map[string]interface{})["creation_time"]

	if !reflect.DeepEqual(actual["data"], expected) {
		t.Fatalf("\nexpected: %#v\nactual: %#v", expected, actual["data"])
	}
}

func TestSysRootGeneration_ReInitUpdate(t *testing.T) {
	core, master, token := vault.TestCoreUnsealed(t)
	ln, addr := TestServer(t, core)
	defer ln.Close()
	TestServerAuth(t, addr, token)

	resp := testHttpPut(t, token, addr+"/v1/sys/root-generation/attempt", nil)
	testResponseStatus(t, resp, 204)

	resp = testHttpDelete(t, token, addr+"/v1/sys/root-generation/attempt")
	testResponseStatus(t, resp, 204)

	resp = testHttpPut(t, token, addr+"/v1/sys/root-generation/attempt", nil)

	resp = testHttpPut(t, token, addr+"/v1/sys/root-generation/update", map[string]interface{}{
		"key": hex.EncodeToString(master),
	})

	testResponseStatus(t, resp, 400)
}
