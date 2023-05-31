package jamf

func (c *Client) AuthToken() *AuthToken {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.currentToken == nil {
		return nil
	}

	return &AuthToken{
		Token:   c.currentToken.Token,
		Expires: c.currentToken.Expires,
	}
}

func (c *Client) SetAuthToken(token *AuthToken) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if token == nil {
		c.currentToken = nil
		return
	}

	c.currentToken = &AuthToken{
		Token:   token.Token,
		Expires: token.Expires,
	}
}
