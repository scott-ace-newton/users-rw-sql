package persistence

//UserRecord is the model for a User
type UserRecord struct {
	UserID string `json:"userID"`
	FirstName string `json:"firstName"`
	LastName string `json:"lastName"`
	EmailAddress string `json:"emailAddress"`
	Password string `json:"password"`
	NickName string `json:"nickname"`
	Country string `json:"country"`
}

//Message is the model for a message
type Message struct {
	Type string
	UserID string
	Nickname string
}
