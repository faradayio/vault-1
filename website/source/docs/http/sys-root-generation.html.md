---
layout: "http"
page_title: "HTTP API: /sys/root-generation/"
sidebar_current: "docs-http-sys-root-generation"
description: |-
  The `/sys/root-generation/` endpoints are used to create a new root key for Vault.
---

# /sys/root-generation/attempt

## GET

<dl>
  <dt>Description</dt>
  <dd>
      Reads the configuration and progress of the current root generation
      attempt.
  </dd>

  <dt>Method</dt>
  <dd>GET</dd>

  <dt>URL</dt>
  <dd>`/sys/root-generation/attempt`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>

  <dt>Returns</dt>
  <dd>
    If a root generation is started, `progress` is how many unseal keys have
    been provided for this generation attempt, where `required` must be reached
    to complete. The `nonce` for the current attempt is also displayed.
    Whether the attempt is complete is also displayed.

    ```javascript
    {
      "started": true,
      "nonce": "2dbd10f1-8528-6246-09e7-82b25b8aba63",
      "progress": 1,
      "required": 3,
      "complete": false
    }
    ```

  </dd>
</dl>

## PUT

<dl>
  <dt>Description</dt>
  <dd>
    Initializes a new root generation attempt. There are no parameters; the
    token supplied for the attempt will be used as the token ID of the new
    root token generated after a successful attempt. **If there is an existing
    token using that token ID, it will first be revoked.**
    <br/><br/>
    Only a single root generation attempt can take place at a time, and
    changing the token ID of a root generation attempt requires canceling and
    starting a new attempt, which will also provide a new nonce.
  </dd>

  <dt>Method</dt>
  <dd>PUT</dd>

  <dt>URL</dt>
  <dd>`/sys/root-generation/attempt`</dd>

  <dt>Parameters</dt>
  <dd>
    None.
  </dd>

  <dt>Returns</dt>
  <dd>
    The current progress.

    ```javascript
    {
      "started": true,
      "nonce": "2dbd10f1-8528-6246-09e7-82b25b8aba63",
      "progress": 1,
      "required": 3,
      "complete": false
    }
    ```

  </dd>
</dl>

## DELETE

<dl>
  <dt>Description</dt>
  <dd>
    Cancels any in-progress root generation attempt. This clears any progress
    made. This must be called to change the token ID being used.
  </dd>

  <dt>Method</dt>
  <dd>DELETE</dd>

  <dt>URL</dt>
  <dd>`/sys/root-generation/attempt`</dd>

  <dt>Parameters</dt>
  <dd>None
  </dd>

  <dt>Returns</dt>
  <dd>`204` response code.
  </dd>
</dl>

# /sys/root-generation/update

## PUT

<dl>
  <dt>Description</dt>
  <dd>
    Enter a single master key share to progress the root generation attempt.
    If the threshold number of master key shares is reached, Vault will
    complete the root generation and issue the new (or updated) token.
    Otherwise, this API must be called multiple times until that threshold is
    met. The attempt nonce must be provided with each call.
  </dd>

  <dt>Method</dt>
  <dd>PUT</dd>

  <dt>URL</dt>
  <dd>`/sys/root-generation/update`</dd>

  <dt>Parameters</dt>
  <dd>
    <ul>
      <li>
        <span class="param">key</span>
        <span class="param-flags">required</span>
        A single master share key.
      </li>
      <li>
        <span class="param">nonce</span>
        <span class="param-flags">required</span>
        The nonce of the attempt.
      </li>
    </ul>
  </dd>

  <dt>Returns</dt>
  <dd>
    A JSON-encoded object indicating the attempt nonce and completion status.

    ```javascript
    {
      "started": true,
      "nonce": "2dbd10f1-8528-6246-09e7-82b25b8aba63",
      "progress": 3,
      "required": 3,
      "complete": true 
    }
    ```

  </dd>
</dl>
