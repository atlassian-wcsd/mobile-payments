package opendevopslambda

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockedPutOjbect struct {
	s3iface.S3API
	Response s3.PutObjectOutput
}

type mockedPutItem struct {
	dynamodbiface.DynamoDBAPI
	Response dynamodb.PutItemOutput
}

type mockedHTTPClient struct {
	response *http.Response
	err      error
}

func (d mockedPutOjbect) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	return &d.Response, nil
}

func (d mockedPutItem) PutItem(input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	return &d.Response, nil
}

func (m mockedHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.response, m.err
}

func TestHandler(t *testing.T) {
	t.Run("Successful Request", func(t *testing.T) {
		mpo := mockedPutOjbect{
			Response: s3.PutObjectOutput{},
		}

		mpi := mockedPutItem{
			Response: dynamodb.PutItemOutput{},
		}

		d := Dependency{
			DepS3:       mpo,
			DepDynamoDB: mpi,
			DepHTTPGet: func(url string) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("test image")),
				}, nil
			},
		}

		ctx := context.Background()
		lc := new(lambdacontext.LambdaContext)
		lc.InvokedFunctionArn = "arn:aws:lambda:region:123456789000:function:functionName"
		ctx = lambdacontext.NewContext(ctx, lc)

		qsp := map[string]string{}
		qsp["url"] = "https://cdn.britannica.com/05/30105-004-644BE36D.jpg"

		request := events.APIGatewayProxyRequest{
			QueryStringParameters: qsp,
		}

		_, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatal(fmt.Sprintf("TestHandler failed with %s", err.Error()))
		}
	})
}

func TestNotificationHandler(t *testing.T) {
	t.Run("Sends notification when consent is granted", func(t *testing.T) {
		t.Setenv("PUSH_PROVIDER", "FCM")
		t.Setenv("FCM_SERVER_KEY", "test-key")
		t.Setenv("FCM_ENDPOINT", "https://fcm.test/send")

		d := Dependency{
			DepHTTPClient: mockedHTTPClient{
				response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"success":1}`)),
				},
			},
		}

		request := events.APIGatewayProxyRequest{
			Path: "/notifications",
			Body: `{"token":"device-token","title":"Payment update","message":"Payment received","consent":true}`,
		}

		resp, err := d.Handler(context.Background(), request)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Rejects notification when consent is not granted", func(t *testing.T) {
		t.Setenv("PUSH_PROVIDER", "FCM")
		t.Setenv("FCM_SERVER_KEY", "test-key")
		t.Setenv("FCM_ENDPOINT", "https://fcm.test/send")

		d := Dependency{
			DepHTTPClient: mockedHTTPClient{
				response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"success":1}`)),
				},
			},
		}

		request := events.APIGatewayProxyRequest{
			Path: "/notifications",
			Body: `{"token":"device-token","title":"Payment update","message":"Payment received","consent":false}`,
		}

		resp, err := d.Handler(context.Background(), request)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", resp.StatusCode)
		}
	})
}
