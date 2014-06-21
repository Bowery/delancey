// Copyright 2014 Bowery, Inc.
package main

import (
	"github.com/subosito/twilio"
)

const (
	TWILIO_ACCOUNT_SID = "ACb6a7631dfbba4712b32319a03aa270d7"
	TWILIO_AUTH_TOKEN  = "6a710ad22d178a7670405f07f71cc632"
	TWILIO_NUMBER      = "+16466062561"
)

var (
	twilioClient *twilio.Client
)

// Create clients.
func init() {
	twilioClient = twilio.NewClient(TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, nil)
}

// Send a text message to a set of numbers.
func SendText(message string, numbers []string) {
	for _, num := range numbers {
		twilioClient.Messages.SendSMS(TWILIO_NUMBER, num, message)
	}
}
