package opendevopslambda

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type Dependency struct {
	DepS3         s3iface.S3API
	DepDynamoDB   dynamodbiface.DynamoDBAPI
	DepHTTPClient httpClient
	DepHTTPGet    func(string) (*http.Response, error)
}

var bucketRootName = "open-devops-images"

const (
	defaultPushProvider = "FCM"
	defaultFCMEndpoint  = "https://fcm.googleapis.com/fcm/send"
)

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type pushNotificationRequest struct {
	Token   string `json:"token"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Consent bool   `json:"consent"`
}

func (d *Dependency) processRequest(imageUrl string, region string, aws_account_id string) (string, error) {
	httpGet := d.DepHTTPGet
	if httpGet == nil {
		httpGet = http.Get
	}

	response, err := httpGet(imageUrl)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return "", errors.New(fmt.Sprintf("response.StatusCode %d != 200\n", response.StatusCode))
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	bucketName := fmt.Sprintf("%s-%s-%s", bucketRootName, region, aws_account_id)

	imageUuid, uuidErr := uuid.NewRandom()
	if uuidErr != nil {
		return "", uuidErr
	}

	s3Input := &s3.PutObjectInput{
		Body:   bytes.NewReader(data),
		Bucket: aws.String(bucketName),
		Key:    aws.String(imageUuid.String()),
	}

	_, s3err := d.DepS3.PutObject(s3Input)
	if s3err != nil {
		return "", s3err
	}

	dynamoInput := &dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			"Id": {
				S: aws.String(imageUuid.String()),
			},
			"Label": {
				S: aws.String("NOT_CLASSIFIED"),
			},
		},
		TableName: aws.String("ImageLabels"),
	}

	_, dynamoErr := d.DepDynamoDB.PutItem(dynamoInput)
	if dynamoErr != nil {
		return "", dynamoErr
	}

	return imageUuid.String(), nil
}

func (d *Dependency) sendPushNotification(req pushNotificationRequest) error {
	if !req.Consent {
		return errors.New("user consent is required")
	}
	if req.Token == "" || req.Title == "" || req.Message == "" {
		return errors.New("token, title, and message are required")
	}

	pushProvider := os.Getenv("PUSH_PROVIDER")
	if pushProvider == "" {
		pushProvider = defaultPushProvider
	}

	if strings.ToUpper(pushProvider) != defaultPushProvider {
		return fmt.Errorf("unsupported push provider: %s", pushProvider)
	}

	serverKey := os.Getenv("FCM_SERVER_KEY")
	if serverKey == "" {
		return errors.New("missing FCM_SERVER_KEY")
	}

	fcmEndpoint := os.Getenv("FCM_ENDPOINT")
	if fcmEndpoint == "" {
		fcmEndpoint = defaultFCMEndpoint
	}

	payload, err := json.Marshal(map[string]interface{}{
		"to": req.Token,
		"notification": map[string]string{
			"title": req.Title,
			"body":  req.Message,
		},
	})
	if err != nil {
		return err
	}

	httpDep := d.DepHTTPClient
	if httpDep == nil {
		httpDep = http.DefaultClient
	}

	httpReq, err := http.NewRequest(http.MethodPost, fcmEndpoint, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "key="+serverKey)
	httpReq.Header.Set("Content-Type", "application/json")

	response, err := httpDep.Do(httpReq)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("push provider returned non-success status: %d", response.StatusCode)
	}

	return nil
}

func isValidExtension(urlVal string) bool {
	validExtensions := []string{"jpeg", "jpg", "bmp", "png", "tiff", "gif", "tif"}

	urlSlice := strings.Split(urlVal, "/")
	fileName := urlSlice[len(urlSlice)-1]
	fileNameSlice := strings.Split(fileName, ".")
	fileExtension := fileNameSlice[len(fileNameSlice)-1]

	for _, ext := range validExtensions {
		if fileExtension == ext {
			return true
		}
	}
	return false
}

func (d *Dependency) Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	if request.Path == "/notifications" {
		var pushReq pushNotificationRequest
		if err := json.Unmarshal([]byte(request.Body), &pushReq); err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 400,
				Body:            `{"status":"error","message":"invalid request body"}`,
				IsBase64Encoded: false,
			}, err
		}

		if err := d.sendPushNotification(pushReq); err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500,
				Body:            fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error()),
				IsBase64Encoded: false,
			}, err
		}

		return events.APIGatewayProxyResponse{StatusCode: 200,
			Body:            `{"status":"sent","provider":"FCM"}`,
			IsBase64Encoded: false,
		}, nil
	}

	lc, _ := lambdacontext.FromContext(ctx)
	region := strings.Split(lc.InvokedFunctionArn, ":")[3]
	aws_account_id := strings.Split(lc.InvokedFunctionArn, ":")[4]

	urlParam, found := request.QueryStringParameters["url"]
	if found {
		urlVal, err := url.QueryUnescape(urlParam)
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500,
				Body:            `{"ImageId":"error"}`,
				IsBase64Encoded: false,
			}, err
		}

		if !isValidExtension(urlVal) {
			return events.APIGatewayProxyResponse{StatusCode: 500,
				Body:            `{"ImageId":"error"}`,
				IsBase64Encoded: false,
			}, errors.New("file extension %s is not valid")
		}

		processString, processErr := d.processRequest(urlVal, region, aws_account_id)
		return events.APIGatewayProxyResponse{StatusCode: 200,
			Body:            fmt.Sprintf(`"ImageId":"%s"`, processString),
			IsBase64Encoded: false,
		}, processErr
	}

	return events.APIGatewayProxyResponse{StatusCode: 500,
		Body:            `{"ImageId":"error"}`,
		IsBase64Encoded: false,
	}, errors.New("url parameter not found")
}
