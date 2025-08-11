package api

func (c *Client) AccessToken() *AccessToken {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.currentToken == nil {
		return nil
	}

	return &AccessToken{
		AccessToken: c.currentToken.AccessToken,
		Expires:     c.currentToken.Expires,
	}
}

func (c *Client) SetAccessToken(token *AccessToken) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if token == nil {
		c.currentToken = nil
		return
	}

	c.currentToken = &AccessToken{
		AccessToken: token.AccessToken,
		ExpiresIn:   token.ExpiresIn,
		Expires:     token.Expires,
	}
}
