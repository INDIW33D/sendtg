package entity

// Contact represents a Telegram user contact
type Contact struct {
	ID        int64
	FirstName string
	LastName  string
	Username  string
	Phone     string
}

// FullName returns the full name of the contact
func (c Contact) FullName() string {
	if c.LastName == "" {
		return c.FirstName
	}
	return c.FirstName + " " + c.LastName
}

// DisplayName returns the best available name for display
func (c Contact) DisplayName() string {
	if name := c.FullName(); name != "" {
		return name
	}
	if c.Username != "" {
		return "@" + c.Username
	}
	return c.Phone
}
