package emergent

import "context"

// --- Improvement ---

// CreateImprovement creates a new Improvement entity.
func (c *Client) CreateImprovement(ctx context.Context, improvement *Improvement) (*Improvement, error) {
	props, err := toProps(improvement)
	if err != nil {
		return nil, err
	}

	obj, err := c.CreateObject(ctx, TypeImprovement, nil, props, improvement.Tags)
	if err != nil {
		return nil, err
	}

	return fromProps[Improvement](obj)
}

// GetImprovement retrieves an Improvement by ID.
func (c *Client) GetImprovement(ctx context.Context, id string) (*Improvement, error) {
	obj, err := c.GetObject(ctx, id)
	if err != nil {
		return nil, err
	}

	return fromProps[Improvement](obj)
}

// UpdateImprovement updates an existing Improvement entity.
func (c *Client) UpdateImprovement(ctx context.Context, id string, improvement *Improvement) (*Improvement, error) {
	props, err := toProps(improvement)
	if err != nil {
		return nil, err
	}

	obj, err := c.UpdateObject(ctx, id, props, improvement.Tags)
	if err != nil {
		return nil, err
	}

	return fromProps[Improvement](obj)
}

// DeleteImprovement deletes an Improvement by ID.
func (c *Client) DeleteImprovement(ctx context.Context, id string) error {
	return c.DeleteObject(ctx, id)
}
