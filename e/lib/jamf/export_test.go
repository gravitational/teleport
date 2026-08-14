package jamf

import "context"

func (c *Client) AuthToken() *AuthToken {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.currentToken == nil {
		return nil
	}

	return &AuthToken{
		Token:   c.currentToken.GetAccessToken(),
		Expires: c.currentToken.GetExpires(),
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

func (c *Client) GetV2ComputersInventory(
	ctx context.Context, req *GetComputersInventoryRequest) (*GetComputersInventoryResponse, error) {
	return c.getV2ComputersInventory(ctx, req)
}

func (c *Client) GetV2ComputersInventoryByID(
	ctx context.Context, req *GetComputersInventoryByIDRequest) (*ComputerInventory, error) {
	return c.getV2ComputersInventoryByID(ctx, req)
}
