package main

import (
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"submit-image/opendevopslambda"
	"submit-image/payment"
)

func init() {
	log.SetOutput(os.Stdout)
}

func main() {
	sess := session.Must(session.NewSession())

	handlerMode := os.Getenv("HANDLER_MODE")

	switch handlerMode {
	case "payment":
		d := payment.Dependency{
			DepDynamoDB: dynamodb.New(sess),
		}
		lambda.Start(d.Handler)
	default:
		d := opendevopslambda.Dependency{
			DepS3:       s3.New(sess),
			DepDynamoDB: dynamodb.New(sess),
		}
		lambda.Start(d.Handler)
	}
}
