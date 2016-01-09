package api

func (c *Sys) RootGenerationStatus() (*RootGenerationStatusResponse, error) {
	r := c.c.NewRequest("GET", "/v1/sys/root-generation/attempt")
	resp, err := c.c.RawRequest(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result RootGenerationStatusResponse
	err = resp.DecodeJSON(&result)
	return &result, err
}

func (c *Sys) RootGenerationInit() error {
	r := c.c.NewRequest("PUT", "/v1/sys/root-generation/attempt")

	resp, err := c.c.RawRequest(r)
	if err == nil {
		defer resp.Body.Close()
	}
	return err
}

func (c *Sys) RootGenerationCancel() error {
	r := c.c.NewRequest("DELETE", "/v1/sys/root-generation/attempt")
	resp, err := c.c.RawRequest(r)
	if err == nil {
		defer resp.Body.Close()
	}
	return err
}

func (c *Sys) RootGenerationUpdate(shard, nonce string) (*RootGenerationStatusResponse, error) {
	body := map[string]interface{}{
		"key":   shard,
		"nonce": nonce,
	}

	r := c.c.NewRequest("PUT", "/v1/sys/root-generation/update")
	if err := r.SetJSONBody(body); err != nil {
		return nil, err
	}

	resp, err := c.c.RawRequest(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result RootGenerationStatusResponse
	err = resp.DecodeJSON(&result)
	return &result, err
}

type RootGenerationStatusResponse struct {
	Nonce    string
	Started  bool
	Progress int
	Required int
	Complete bool
}
