package notification

import (
	"fmt"
	"github.com/scott-ace-newton/users-rw-sql/persistence"
)

type QueueClient struct {
	queueURL string
}

func NewQueueClient(queueURL string) QueueClient {
	return QueueClient{queueURL}
}

func(qc *QueueClient) AddMessageToQueue(msg persistence.Message) {
	fmt.Printf("adding message to queue: %v", msg)
}
