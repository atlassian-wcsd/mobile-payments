package opendevopslambda

import (
	"bytes"
	"context"
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
	"strings"
)

type Dependency struct {
	DepS3 s3iface.S3API
	DepDynamoDB dynamodbiface.DynamoDBAPI
}

var bucketRootName = "open-devops-images"

func (d *Dependency) processRequest(imageUrl string, region string, aws_account_id string) (string, error) {
	response, err := http.Get(imageUrl)
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
	path := request.Path
	httpMethod := request.HTTPMethod

	// Route payment-related requests
	if strings.HasPrefix(path, "/payments") || strings.HasPrefix(path, "/Prod/payments") {
		return d.routePaymentRequest(path, httpMethod, request)
	}

	// Default: original image submission handler
	return d.handleImageSubmission(ctx, request)
}

// routePaymentRequest routes incoming requests to the appropriate payment handler.
func (d *Dependency) routePaymentRequest(path string, httpMethod string, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Normalize path by removing /Prod prefix if present
	normalizedPath := path
	if strings.HasPrefix(normalizedPath, "/Prod") {
		normalizedPath = strings.TrimPrefix(normalizedPath, "/Prod")
	}

	switch {
	// GET /payments/methods - List all payment methods
	case normalizedPath == "/payments/methods" && httpMethod == "GET":
		return HandleListPaymentMethods()

	// GET /payments/methods/{id} - Get expanded details for a specific payment method
	case strings.HasPrefix(normalizedPath, "/payments/methods/") && httpMethod == "GET":
		methodID := strings.TrimPrefix(normalizedPath, "/payments/methods/")
		return HandleGetPaymentMethod(methodID)

	// GET /payments/redirect/{type} - Get redirect URL for a payment method
	case strings.HasPrefix(normalizedPath, "/payments/redirect/") && httpMethod == "GET":
		methodType := strings.TrimPrefix(normalizedPath, "/payments/redirect/")
		return HandlePaymentRedirect(methodType, request.QueryStringParameters)

	default:
		return errorResponse(404, fmt.Sprintf("payment route not found: %s %s", httpMethod, normalizedPath))
	}
}

// handleImageSubmission processes the original image submission logic.
func (d *Dependency) handleImageSubmission(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)
	region := strings.Split(lc.InvokedFunctionArn, ":")[3]
	aws_account_id := strings.Split(lc.InvokedFunctionArn, ":")[4]

	urlParam, found := request.QueryStringParameters["url"]
	if found {
		urlVal, err := url.QueryUnescape(urlParam)
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500,
				Body: `{"ImageId":"error"}`,
				IsBase64Encoded: false,
			}, err
		}

		if !isValidExtension(urlVal) {
			return events.APIGatewayProxyResponse{StatusCode: 500,
				Body: `{"ImageId":"error"}`,
				IsBase64Encoded: false,
			}, errors.New("file extension %s is not valid")
		}

		processString, processErr := d.processRequest(urlVal, region, aws_account_id)
		return events.APIGatewayProxyResponse{StatusCode: 200,
			Body: fmt.Sprintf(`"ImageId":"%s"`, processString),
			IsBase64Encoded: false,
		}, processErr
	}

	return events.APIGatewayProxyResponse{StatusCode: 500,
		Body: `{"ImageId":"error"}`,
		IsBase64Encoded: false,
	}, errors.New("url parameter not found")
}
