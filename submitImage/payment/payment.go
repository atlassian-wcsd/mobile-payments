package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

// PaymentMethodType represents the category of a payment method.
type PaymentMethodType string

const (
	TypeCard            PaymentMethodType = "card"
	TypeInternetBanking PaymentMethodType = "internet_banking"
	TypeThirdPartyApp   PaymentMethodType = "third_party_app"
	TypePayPal          PaymentMethodType = "paypal"
	TypeWallet          PaymentMethodType = "wallet"
)

// PaymentMethod represents an available payment method with metadata.
type PaymentMethod struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Type            PaymentMethodType `json:"type"`
	Icon            string            `json:"icon"`
	Description     string            `json:"description"`
	RedirectURL     string            `json:"redirect_url,omitempty"`
	IsExpandable    bool              `json:"is_expandable"`
	ExpandedDetails *ExpandedDetails  `json:"expanded_details,omitempty"`
	Enabled         bool              `json:"enabled"`
}

// ExpandedDetails provides additional information shown when a payment method is expanded.
type ExpandedDetails struct {
	Instructions   string   `json:"instructions,omitempty"`
	Fields         []string `json:"fields,omitempty"`
	SupportedBanks []string `json:"supported_banks,omitempty"`
	ProcessingTime string   `json:"processing_time,omitempty"`
	Fees           string   `json:"fees,omitempty"`
}

// RedirectResponse is returned when a user selects a payment method requiring redirection.
type RedirectResponse struct {
	PaymentMethodID string `json:"payment_method_id"`
	RedirectURL     string `json:"redirect_url"`
	SessionToken    string `json:"session_token,omitempty"`
	ExpiresIn       int    `json:"expires_in,omitempty"`
}

// PaymentMethodsResponse is the response for listing payment methods.
type PaymentMethodsResponse struct {
	PaymentMethods []PaymentMethod `json:"payment_methods"`
	Total          int             `json:"total"`
}

// ErrorResponse represents an error response payload.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

const paymentMethodsTable = "PaymentMethods"

// Dependency holds the external service dependencies for the payment handler.
type Dependency struct {
	DepDynamoDB dynamodbiface.DynamoDBAPI
}

// getDefaultPaymentMethods returns the built-in set of supported payment methods.
func getDefaultPaymentMethods() []PaymentMethod {
	return []PaymentMethod{
		{
			ID:           "pm_visa",
			Name:         "Visa",
			Type:         TypeCard,
			Icon:         "https://cdn.payments.example.com/icons/visa.svg",
			Description:  "Pay with Visa credit or debit card",
			IsExpandable: true,
			Enabled:      true,
			ExpandedDetails: &ExpandedDetails{
				Fields:         []string{"card_number", "expiry_date", "cvv", "cardholder_name"},
				ProcessingTime: "Instant",
				Fees:           "No additional fees",
			},
		},
		{
			ID:           "pm_mastercard",
			Name:         "Mastercard",
			Type:         TypeCard,
			Icon:         "https://cdn.payments.example.com/icons/mastercard.svg",
			Description:  "Pay with Mastercard credit or debit card",
			IsExpandable: true,
			Enabled:      true,
			ExpandedDetails: &ExpandedDetails{
				Fields:         []string{"card_number", "expiry_date", "cvv", "cardholder_name"},
				ProcessingTime: "Instant",
				Fees:           "No additional fees",
			},
		},
		{
			ID:           "pm_amex",
			Name:         "American Express",
			Type:         TypeCard,
			Icon:         "https://cdn.payments.example.com/icons/amex.svg",
			Description:  "Pay with American Express card",
			IsExpandable: true,
			Enabled:      true,
			ExpandedDetails: &ExpandedDetails{
				Fields:         []string{"card_number", "expiry_date", "cvv", "cardholder_name"},
				ProcessingTime: "Instant",
				Fees:           "No additional fees",
			},
		},
		{
			ID:          "pm_internet_banking",
			Name:        "Internet Banking",
			Type:        TypeInternetBanking,
			Icon:        "https://cdn.payments.example.com/icons/internet-banking.svg",
			Description: "Pay via your bank's internet banking portal",
			RedirectURL: "https://banking.example.com/pay",
			IsExpandable: true,
			Enabled:      true,
			ExpandedDetails: &ExpandedDetails{
				Instructions:   "You will be redirected to your bank's secure internet banking portal to complete the payment.",
				SupportedBanks: []string{"Chase", "Bank of America", "Wells Fargo", "Citibank", "US Bank"},
				ProcessingTime: "1-2 business days",
				Fees:           "No additional fees",
			},
		},
		{
			ID:          "pm_third_party_banking",
			Name:        "Banking App",
			Type:        TypeThirdPartyApp,
			Icon:        "https://cdn.payments.example.com/icons/banking-app.svg",
			Description: "Pay using a third-party banking app",
			RedirectURL: "https://thirdpartybank.example.com/authorize",
			IsExpandable: true,
			Enabled:      true,
			ExpandedDetails: &ExpandedDetails{
				Instructions:   "You will be redirected to the banking app to authorize the payment.",
				ProcessingTime: "Instant to 1 business day",
				Fees:           "May vary by bank",
			},
		},
		{
			ID:          "pm_paypal",
			Name:        "PayPal",
			Type:        TypePayPal,
			Icon:        "https://cdn.payments.example.com/icons/paypal.svg",
			Description: "Pay with your PayPal account",
			RedirectURL: "https://www.paypal.com/checkoutnow",
			IsExpandable: true,
			Enabled:      true,
			ExpandedDetails: &ExpandedDetails{
				Instructions:   "You will be redirected to PayPal to log in and confirm your payment.",
				ProcessingTime: "Instant",
				Fees:           "No additional fees for buyers",
			},
		},
		{
			ID:           "pm_apple_pay",
			Name:         "Apple Pay",
			Type:         TypeWallet,
			Icon:         "https://cdn.payments.example.com/icons/apple-pay.svg",
			Description:  "Pay with Apple Pay",
			IsExpandable: true,
			Enabled:      true,
			ExpandedDetails: &ExpandedDetails{
				Instructions:   "Authenticate with Face ID or Touch ID to complete payment.",
				ProcessingTime: "Instant",
				Fees:           "No additional fees",
			},
		},
		{
			ID:           "pm_google_pay",
			Name:         "Google Pay",
			Type:         TypeWallet,
			Icon:         "https://cdn.payments.example.com/icons/google-pay.svg",
			Description:  "Pay with Google Pay",
			IsExpandable: true,
			Enabled:      true,
			ExpandedDetails: &ExpandedDetails{
				Instructions:   "Confirm payment through your Google Pay account.",
				ProcessingTime: "Instant",
				Fees:           "No additional fees",
			},
		},
	}
}

// jsonResponse creates an API Gateway proxy response with the given status code and body.
func jsonResponse(statusCode int, body interface{}) (events.APIGatewayProxyResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error":"internal_error","message":"failed to serialize response"}`,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       string(jsonBody),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

// errorResponse creates an error API Gateway proxy response.
func errorResponse(statusCode int, errCode string, message string) (events.APIGatewayProxyResponse, error) {
	return jsonResponse(statusCode, ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}

// listPaymentMethods returns all available payment methods, optionally filtered by type.
func (d *Dependency) listPaymentMethods(filterType string) ([]PaymentMethod, error) {
	methods := getDefaultPaymentMethods()

	if filterType == "" {
		return methods, nil
	}

	filtered := make([]PaymentMethod, 0)
	for _, m := range methods {
		if string(m.Type) == filterType {
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
}

// getPaymentMethodByID returns a single payment method by its ID.
func (d *Dependency) getPaymentMethodByID(id string) (*PaymentMethod, error) {
	methods := getDefaultPaymentMethods()
	for _, m := range methods {
		if m.ID == id {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("payment method not found: %s", id)
}

// getExpandedView returns a payment method with its expanded details populated.
func (d *Dependency) getExpandedView(id string) (*PaymentMethod, error) {
	method, err := d.getPaymentMethodByID(id)
	if err != nil {
		return nil, err
	}
	if !method.IsExpandable {
		return nil, fmt.Errorf("payment method %s does not support expanded view", id)
	}
	return method, nil
}

// getRedirectURL returns the redirect information for a payment method that requires external redirection.
func (d *Dependency) getRedirectURL(id string) (*RedirectResponse, error) {
	method, err := d.getPaymentMethodByID(id)
	if err != nil {
		return nil, err
	}
	if method.RedirectURL == "" {
		return nil, fmt.Errorf("payment method %s does not support redirection", id)
	}
	return &RedirectResponse{
		PaymentMethodID: method.ID,
		RedirectURL:     method.RedirectURL,
		ExpiresIn:       300, // 5 minutes
	}, nil
}

// recordPaymentMethodSelection stores the user's payment method selection in DynamoDB.
func (d *Dependency) recordPaymentMethodSelection(userID string, methodID string) error {
	input := &dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			"UserId": {
				S: aws.String(userID),
			},
			"PaymentMethodId": {
				S: aws.String(methodID),
			},
			"Status": {
				S: aws.String("SELECTED"),
			},
		},
		TableName: aws.String(paymentMethodsTable),
	}
	_, err := d.DepDynamoDB.PutItem(input)
	return err
}

// Handler handles API Gateway requests for payment method operations.
// Routes:
//   GET  /payment/methods           - List all payment methods (optional ?type= filter)
//   GET  /payment/methods/{id}      - Get a specific payment method
//   GET  /payment/methods/{id}/expand - Get expanded view of a payment method
//   GET  /payment/methods/{id}/redirect - Get redirect URL for a payment method
//   POST /payment/methods/{id}/select  - Record a payment method selection
func (d *Dependency) Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	path := request.Path
	httpMethod := request.HTTPMethod

	// Normalize path: remove trailing slash
	path = strings.TrimRight(path, "/")

	// Route: GET /payment/methods
	if (path == "/payment/methods" || path == "/Prod/payment/methods") && httpMethod == "GET" {
		filterType := request.QueryStringParameters["type"]
		methods, err := d.listPaymentMethods(filterType)
		if err != nil {
			return errorResponse(http.StatusInternalServerError, "list_error", err.Error())
		}
		return jsonResponse(http.StatusOK, PaymentMethodsResponse{
			PaymentMethods: methods,
			Total:          len(methods),
		})
	}

	// Extract payment method ID from path
	methodID := extractMethodID(path)

	if methodID == "" {
		return errorResponse(http.StatusBadRequest, "bad_request", "invalid path or missing payment method ID")
	}

	// Route: GET /payment/methods/{id}/expand
	if strings.HasSuffix(path, "/expand") && httpMethod == "GET" {
		method, err := d.getExpandedView(methodID)
		if err != nil {
			return errorResponse(http.StatusNotFound, "not_found", err.Error())
		}
		return jsonResponse(http.StatusOK, method)
	}

	// Route: GET /payment/methods/{id}/redirect
	if strings.HasSuffix(path, "/redirect") && httpMethod == "GET" {
		redirect, err := d.getRedirectURL(methodID)
		if err != nil {
			if strings.Contains(err.Error(), "does not support redirection") {
				return errorResponse(http.StatusBadRequest, "no_redirect", err.Error())
			}
			return errorResponse(http.StatusNotFound, "not_found", err.Error())
		}
		return jsonResponse(http.StatusOK, redirect)
	}

	// Route: POST /payment/methods/{id}/select
	if strings.HasSuffix(path, "/select") && httpMethod == "POST" {
		var body struct {
			UserID string `json:"user_id"`
		}
		if err := json.Unmarshal([]byte(request.Body), &body); err != nil || body.UserID == "" {
			return errorResponse(http.StatusBadRequest, "bad_request", "user_id is required in request body")
		}

		// Validate payment method exists
		_, err := d.getPaymentMethodByID(methodID)
		if err != nil {
			return errorResponse(http.StatusNotFound, "not_found", err.Error())
		}

		if err := d.recordPaymentMethodSelection(body.UserID, methodID); err != nil {
			return errorResponse(http.StatusInternalServerError, "storage_error", "failed to record payment method selection")
		}

		return jsonResponse(http.StatusOK, map[string]string{
			"status":  "success",
			"message": "payment method selected",
		})
	}

	// Route: GET /payment/methods/{id}
	if httpMethod == "GET" && !strings.HasSuffix(path, "/expand") && !strings.HasSuffix(path, "/redirect") && !strings.HasSuffix(path, "/select") {
		method, err := d.getPaymentMethodByID(methodID)
		if err != nil {
			return errorResponse(http.StatusNotFound, "not_found", err.Error())
		}
		return jsonResponse(http.StatusOK, method)
	}

	return errorResponse(http.StatusNotFound, "not_found", "route not found")
}

// extractMethodID extracts the payment method ID from the request path.
// Supports paths like:
//   /payment/methods/{id}
//   /payment/methods/{id}/expand
//   /payment/methods/{id}/redirect
//   /payment/methods/{id}/select
//   /Prod/payment/methods/{id}...
func extractMethodID(path string) string {
	// Remove known prefixes
	path = strings.TrimPrefix(path, "/Prod")

	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Expected: ["payment", "methods", "{id}"] or ["payment", "methods", "{id}", "action"]
	if len(parts) < 3 {
		return ""
	}
	if parts[0] != "payment" || parts[1] != "methods" {
		return ""
	}
	return parts[2]
}

// isValidPaymentMethodType checks if the given type string is a recognized PaymentMethodType.
func isValidPaymentMethodType(t string) bool {
	switch PaymentMethodType(t) {
	case TypeCard, TypeInternetBanking, TypeThirdPartyApp, TypePayPal, TypeWallet:
		return true
	}
	return false
}

// GetPaymentMethodIcon returns the icon URL for a given payment method ID.
// Returns an error if the payment method is not found.
func GetPaymentMethodIcon(methodID string) (string, error) {
	methods := getDefaultPaymentMethods()
	for _, m := range methods {
		if m.ID == methodID {
			if m.Icon == "" {
				return "", errors.New("no icon available for payment method: " + methodID)
			}
			return m.Icon, nil
		}
	}
	return "", errors.New("payment method not found: " + methodID)
}
