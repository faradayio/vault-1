package command

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/vault"
	"github.com/mitchellh/cli"
)

func TestGenerateRoot(t *testing.T) {
	core, ts, key, _ := vault.TestCoreWithTokenStore(t)
	ln, addr := http.TestServer(t, core)
	defer ln.Close()

	ui := new(cli.MockUi)
	c := &GenerateRootCommand{
		Key: hex.EncodeToString(key),
		Meta: Meta{
			Ui:          ui,
			ClientToken: "asdf",
		},
	}

	// Init the attempt
	args := []string{
		"-address", addr,
		"-init",
	}
	if code := c.Run(args); code != 0 {
		t.Fatalf("bad: %d\n\n%s", code, ui.ErrorWriter.String())
	}

	config, err := core.RootGenerationConfiguration()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	c.Nonce = config.Nonce

	// Provide the key
	args = []string{
		"-address", addr,
	}
	if code := c.Run(args); code != 0 {
		t.Fatalf("bad: %d\n\n%s", code, ui.ErrorWriter.String())
	}

	req := logical.TestRequest(t, logical.ReadOperation, "lookup-self")
	req.ClientToken = "asdf"

	resp, err := ts.HandleRequest(req)
	if err != nil {
		t.Fatalf("error running \"asdf\" token lookup-self: %v", err)
	}
	if resp == nil {
		t.Fatalf("got nil resp with \"asdf\" token lookup-self")
	}
	if resp.Data == nil {
		t.Fatalf("got nil resp.Data with \"asdf\" token lookup-self")
	}

	if resp.Data["orphan"].(bool) != true ||
		resp.Data["ttl"].(int64) != 0 ||
		resp.Data["num_uses"].(int) != 0 ||
		resp.Data["meta"].(map[string]string) != map[string]string(nil) ||
		len(resp.Data["policies"].([]string)) != 1 ||
		resp.Data["policies"].([]string)[0] != "root" {
		t.Fatalf("bad: %#v", resp.Data)
	}
}

func TestGenerateRoot_Cancel(t *testing.T) {
	core, key, _ := vault.TestCoreUnsealed(t)
	ln, addr := http.TestServer(t, core)
	defer ln.Close()

	ui := new(cli.MockUi)
	c := &GenerateRootCommand{
		Key: hex.EncodeToString(key),
		Meta: Meta{
			Ui: ui,
		},
	}

	args := []string{"-address", addr, "-init"}
	if code := c.Run(args); code != 0 {
		t.Fatalf("bad: %d\n\n%s", code, ui.ErrorWriter.String())
	}

	args = []string{"-address", addr, "-cancel"}
	if code := c.Run(args); code != 0 {
		t.Fatalf("bad: %d\n\n%s", code, ui.ErrorWriter.String())
	}

	config, err := core.RootGenerationConfiguration()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if config != nil {
		t.Fatal("should not have a config for root generation")
	}
}

func TestGenerateRoot_status(t *testing.T) {
	core, key, _ := vault.TestCoreUnsealed(t)
	ln, addr := http.TestServer(t, core)
	defer ln.Close()

	ui := new(cli.MockUi)
	c := &GenerateRootCommand{
		Key: hex.EncodeToString(key),
		Meta: Meta{
			Ui: ui,
		},
	}

	args := []string{"-address", addr, "-init"}
	if code := c.Run(args); code != 0 {
		t.Fatalf("bad: %d\n\n%s", code, ui.ErrorWriter.String())
	}

	args = []string{"-address", addr, "-status"}
	if code := c.Run(args); code != 0 {
		t.Fatalf("bad: %d\n\n%s", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(string(ui.OutputWriter.Bytes()), "Started: true") {
		t.Fatalf("bad: %s", ui.OutputWriter.String())
	}
}
