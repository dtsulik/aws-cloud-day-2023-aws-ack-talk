package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func main() {
	env, ok := os.LookupEnv("MODE")
	queue, qok := os.LookupEnv("QUEUE")
	if !qok {
		log.Fatal("QUEUE URL not provided")
	}

	if !ok {
		log.Fatal("MODE env var not set")
	}

	c := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		s := <-c
		log.Printf("Received signal: %s. Quiting", s.String())
		done <- true
	}()

	var wg sync.WaitGroup

	if env == "send" {
		wg.Add(1)
		go func(q string) {
			t := time.NewTicker(5 * time.Second)
			for {
				log.Printf("Working as %s with queue: %s\n", env, q)
				send(q)
				select {
				case <-t.C:
				case <-done:
					{
						wg.Done()
						return
					}
				}
			}
		}(queue)
	} else if env == "recv" {
		wg.Add(1)
		go func(q string) {
			t := time.NewTicker(5 * time.Second)
			for {
				log.Printf("Working as %s with queue: %s\n", env, q)
				recv(q)
				select {
				case <-t.C:
				case <-done:
					{
						wg.Done()
						return
					}
				}
			}
		}(queue)
	} else {
		log.Fatal("MODE can only be send or recv")
	}

	wg.Wait()
}

func send(queue_name string) {
	client, queue_url := get_client_with_url(queue_name)

	message := &sqs.SendMessageInput{
		DelaySeconds: 10,
		MessageAttributes: map[string]types.MessageAttributeValue{
			"Title": {
				DataType:    aws.String("String"),
				StringValue: aws.String("Thank you ACK"),
			},
			"Author": {
				DataType:    aws.String("String"),
				StringValue: aws.String("Awesome AWS community"),
			},
		},
		MessageBody: aws.String("Building things is fun."),
		QueueUrl:    queue_url,
	}

	resp, err := client.SendMessage(context.TODO(), message)
	if err != nil {
		log.Println("Got an error sending the message:")
		log.Println(err)
		return
	}

	log.Println("Sent message with ID: " + *resp.MessageId)
}

func recv(queue_name string) {
	client, queue_url := get_client_with_url(queue_name)

	message_req := sqs.ReceiveMessageInput{
		MessageAttributeNames: []string{
			string(types.QueueAttributeNameAll),
		},
		QueueUrl:            queue_url,
		MaxNumberOfMessages: 5,
		VisibilityTimeout:   60,
	}

	message, err := client.ReceiveMessage(context.TODO(), &message_req)
	if err != nil {
		log.Fatalln("Failed to recv message: " + err.Error())
	}

	if message.Messages != nil {
		for _, m := range message.Messages {
			log.Printf("Recevied message:\n\tID: %s\n\tReceipt Handle: %s\n\tBody: %s",
				*m.MessageId, *m.ReceiptHandle, *m.Body)
			
			msg_del_req := &sqs.DeleteMessageInput{
				QueueUrl:      queue_url,
				ReceiptHandle: m.ReceiptHandle,
			}
			
			_, err := client.DeleteMessage(context.TODO(), msg_del_req)
			if err != nil {
				log.Printf("Failed to delete message. ID: %s Handle: %s\n", *m.MessageId, *m.ReceiptHandle)
			}
		}
	} else {
		log.Println("Waiting for messages")
	}
}

func get_client_with_url(queue string) (*sqs.Client, *string) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal("configuration error, " + err.Error())
	}

	client := sqs.NewFromConfig(cfg)

	queue_url_req := &sqs.GetQueueUrlInput{
		QueueName: &queue,
	}

	url_res, err := client.GetQueueUrl(context.TODO(), queue_url_req)
	if err != nil {
		log.Fatalln("Got an error getting the queue URL: " + err.Error())
	}

	queue_url := url_res.QueueUrl

	log.Printf("Target queue url: %s", *queue_url)

	return client, queue_url
}
