package notification

import (
	"fmt"
	"github.com/scott-ace-newton/users-rw-sql/persistence"
)

//QueueClient is a simple queue client
type QueueClient struct {
	queueURL string
}

//NewQueueClient returns simple queue client
func NewQueueClient(queueURL string) QueueClient {
	return QueueClient{queueURL}
}

//AddMessageToQueue would add the provided message to the configured queue.
//As this is a test application however it simply prints the message to the terminal
func(qc *QueueClient) AddMessageToQueue(msg persistence.Message) {
	fmt.Printf("adding message to queue: %v\n", msg)
}

//QueueIsWritable is the healthcheck of the configured queue.
//As this is a test application however it simply hardcoded to return true
func(qc *QueueClient) QueueIsWritable() bool {
	return true
}
