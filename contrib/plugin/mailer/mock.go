package mailer

// MockMailer is a test-friendly mailer that records sent messages in memory.
type MockMailer struct {
	Sent []Message
}

// Message represents a sent email recorded by MockMailer.
type Message struct {
	To      string
	Subject string
	Body    string
}

// NewMockMailer returns an empty MockMailer.
func NewMockMailer() *MockMailer {
	return &MockMailer{Sent: make([]Message, 0)}
}

// Send records the message instead of sending over the network.
func (m *MockMailer) Send(to, subject, body string) error {
	if m == nil {
		return nil
	}
	m.Sent = append(m.Sent, Message{To: to, Subject: subject, Body: body})
	return nil
}
