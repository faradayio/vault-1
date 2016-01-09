package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/helper/password"
)

// GenerateRootCommand is a Command that generates a new root token.
type GenerateRootCommand struct {
	Meta

	// Key can be used to pre-seed the key. If it is set, it will not
	// be asked with the `password` helper.
	Key string

	// The nonce for the rekey request to send along
	Nonce string
}

func (c *GenerateRootCommand) Run(args []string) int {
	var init, cancel, status bool
	var nonce string
	flags := c.Meta.FlagSet("generate-root", FlagSetDefault)
	flags.BoolVar(&init, "init", false, "")
	flags.BoolVar(&cancel, "cancel", false, "")
	flags.BoolVar(&status, "status", false, "")
	flags.StringVar(&nonce, "nonce", "", "")
	flags.Usage = func() { c.Ui.Error(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	if nonce != "" {
		c.Nonce = nonce
	}

	client, err := c.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf(
			"Error initializing client: %s", err))
		return 2
	}

	// Check if we are running doing any restricted variants
	switch {
	case init:
		return c.initRootGeneration(client)
	case cancel:
		return c.cancelRootGeneration(client)
	case status:
		return c.rootGenerationStatus(client)
	}

	// Check if the root generation is started
	rootGenerationStatus, err := client.Sys().RootGenerationStatus()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error reading root generation status: %s", err))
		return 1
	}

	// Start the root generation process if not started
	if !rootGenerationStatus.Started {
		err := client.Sys().RootGenerationInit()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error initializing root generation: %s", err))
			return 1
		}
		rootGenerationStatus, err = client.Sys().RootGenerationStatus()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error reading root generation status: %s", err))
			return 1
		}
		c.Nonce = rootGenerationStatus.Nonce
	}

	serverNonce := rootGenerationStatus.Nonce

	// Get the unseal key
	args = flags.Args()
	key := c.Key
	if len(args) > 0 {
		key = args[0]
	}
	if key == "" {
		c.Nonce = serverNonce
		fmt.Printf("Root generation operation nonce: %s\n", serverNonce)
		fmt.Printf("Key (will be hidden): ")
		key, err = password.Read(os.Stdin)
		fmt.Printf("\n")
		if err != nil {
			c.Ui.Error(fmt.Sprintf(
				"Error attempting to ask for password. The raw error message\n"+
					"is shown below, but the most common reason for this error is\n"+
					"that you attempted to pipe a value into unseal or you're\n"+
					"executing `vault generate-root` from outside of a terminal.\n\n"+
					"You should use `vault generate-root` from a terminal for maximum\n"+
					"security. If this isn't an option, the unseal key can be passed\n"+
					"in using the first parameter.\n\n"+
					"Raw error: %s", err))
			return 1
		}
	}

	// Provide the key, this may potentially complete the update
	statusResp, err := client.Sys().RootGenerationUpdate(strings.TrimSpace(key), c.Nonce)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error attempting generate-root update: %s", err))
		return 1
	}

	c.dumpStatus(statusResp)

	return 0
}

// initRootGeneration is used to start the generation process
func (c *GenerateRootCommand) initRootGeneration(client *api.Client) int {
	// Start the rekey
	err := client.Sys().RootGenerationInit()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing root generation: %s", err))
		return 1
	}

	// Provide the current status
	return c.rootGenerationStatus(client)
}

// cancelRootGeneration is used to abort the generation process
func (c *GenerateRootCommand) cancelRootGeneration(client *api.Client) int {
	err := client.Sys().RootGenerationCancel()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to cancel root generation: %s", err))
		return 1
	}
	c.Ui.Output("Root generation canceled.")
	return 0
}

// rootGenerationStatus is used just to fetch and dump the status
func (c *GenerateRootCommand) rootGenerationStatus(client *api.Client) int {
	// Check the status
	status, err := client.Sys().RootGenerationStatus()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error reading root generation status: %s", err))
		return 1
	}

	c.dumpStatus(status)

	return 0
}

// dumpStatus dumps the status to output
func (c *GenerateRootCommand) dumpStatus(status *api.RootGenerationStatusResponse) {
	// Dump the status
	statString := fmt.Sprintf(
		"Nonce: %s\n"+
			"Started: %v\n"+
			"Rekey Progress: %d\n"+
			"Required Keys: %d\n"+
			"Complete: %t",
		status.Nonce,
		status.Started,
		status.Progress,
		status.Required,
		status.Complete,
	)
	c.Ui.Output(statString)
}

func (c *GenerateRootCommand) Synopsis() string {
	return "Promotes a token to a root token"
}

func (c *GenerateRootCommand) Help() string {
	helpText := `
Usage: vault generate-root [options] [key]

  'generate-root' is used to create a new root token. It does this by promoting
  a token value to a root token. This should normally only be done if the root
  token is lost.

  Root generation can only be done when the Vault is already unsealed. The
  operation is done online, but requires that a threshold of the current unseal
  keys be provided. The token that is promoted to a root token is the claimed
  token value of the client that started the process, which does not have to
  exist; however, if it does exist, the token will be revoked before promotion,
  so any resources created by that token (for instance, child tokens) will be
  revoked as well.

General Options:

  ` + generalOptionsUsage() + `

Rekey Options:

  -init                   Initialize the root generation operation. This can
                          only be done if no generation is already initiated.

  -cancel                 Reset the root generation process by throwing away
                          prior unseal keys and the configuration.

  -status                 Prints the status of the current operation. This can
                          be used to see the status without attempting to
                          provide an unseal key.

  -nonce=abcd             The nonce provided at initialization time. This same
                          nonce value must be provided with each unseal key. If
                          the unseal key is not being passed in via the command
                          line the nonce parameter is not required, and will
                          instead be displayed with the key prompt.
`
	return strings.TrimSpace(helpText)
}
