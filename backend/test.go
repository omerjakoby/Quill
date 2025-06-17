package main

import (
	"fmt"
	"time"
)

// producer is a goroutine that sends numbers to a channel.
// It stops after sending 'numToSend' numbers or if the 'done' channel is closed.
func producer(data chan<- int, done <-chan struct{}, numToSend int) {
	fmt.Println("Producer: Starting production...")
	for i := 0; i < numToSend; i++ {
		select {
		case data <- i: // Attempt to send the number
			fmt.Printf("Producer: Sent %d\n", i)
			time.Sleep(50 * time.Millisecond) // Simulate some work
		case <-done: // Check if the done signal is received
			fmt.Println("Producer: Received done signal, stopping.")
			return
		}
	}
	close(data) // Close the data channel when done sending all numbers
	fmt.Println("Producer: Finished sending numbers and closed data channel.")
}

// consumer is a goroutine that receives numbers from a channel.
// It stops when the 'data' channel is closed.
func consumer(data <-chan int, done chan<- struct{}) {
	fmt.Println("Consumer: Starting consumption...")
	defer close(done) // Ensure 'done' channel is closed when consumer exits

	for num := range data { // Loop until the 'data' channel is closed
		fmt.Printf("Consumer: Received %d\n", num)
		time.Sleep(100 * time.Millisecond) // Simulate some processing time
	}
	fmt.Println("Consumer: Data channel closed, stopping.")
}

func main() {
	fmt.Println("Main: Application starting...")

	// Create channels
	// data: Used for sending integers from producer to consumer
	dataChannel := make(chan int)
	// done: Used for signaling the producer to stop gracefully
	doneChannel := make(chan struct{}) // An empty struct {} is used for a signal channel

	const numbersToProduce = 10

	// Start the producer goroutine
	// It sends 'numbersToProduce' integers into dataChannel and can be stopped by doneChannel
	go producer(dataChannel, doneChannel, numbersToProduce)

	// Start the consumer goroutine
	// It reads from dataChannel and signals producer to stop via doneChannel upon completion
	go consumer(dataChannel, doneChannel)

	// Give some time for goroutines to work.
	// In a real application, you might use a WaitGroup or more sophisticated
	// synchronization to ensure all goroutines complete before main exits.
	// For this simple demo, a fixed sleep is sufficient to observe the behavior.
	time.Sleep(2 * time.Second) // Adjust as needed to see all output

	fmt.Println("Main: Application finished.")
}
