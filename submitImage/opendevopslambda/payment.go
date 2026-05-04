package opendevopslambda

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
)

// PaymentMethodType represents the type of a payment method.
type PaymentMethodType string

const (
	PaymentMethodCreditCard      PaymentMethodType = "credit_card"
	PaymentMethodDebitCard       PaymentMethodType = "debit_card"
	PaymentMethodInternetBanking PaymentMethodType = "internet_banking"
	PaymentMethodBankingApp      PaymentMethodType = "banking_app"
	PaymentMethodPayPal          PaymentMethodType = "paypal"
)

// PaymentMethod represents a single payment method with its metadata.
type PaymentMethod struct {
	ID          string            `json:"id"`
	Type        PaymentMethodType `json:"type"`
	DisplayName string            `json:"display_name"`
	Description string            `json:"description"`
	IconURL     string            `json:"icon_url"`
	RedirectURL string            `json:"redirect_url,omitempty"`
	IsExpanded  bool              `json:"is_expanded"`
	Fields      []PaymentField    `json:"fields,omitempty"`
}

// PaymentField represents a field within an expanded payment method view.
type PaymentField struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder,omitempty"`
}

// PaymentRedirectResponse represents the response for a redirect request.
type PaymentRedirectResponse struct {
	PaymentMethodType PaymentMethodType `json:"payment_method_type"`
	RedirectURL       string            `json:"redirect_url"`
	SessionID         string            `json:"session_id,omitempty"`
}

// PaymentMethodsResponse represents the response for listing payment methods.
type PaymentMethodsResponse struct {
	PaymentMethods []PaymentMethod `json:"payment_methods"`
	Count          int             `json:"count"`
}

// getPaymentMethods returns all supported payment methods with icons and expand view details.
func getPaymentMethods() []PaymentMethod {
	return []PaymentMethod{
		{
			ID:          "pm_credit_card",
			Type:        PaymentMethodCreditCard,
			DisplayName: "Credit Card",
			Description: "Pay using Visa, Mastercard, or American Express",
			IconURL:     "/icons/credit_card.svg",
			IsExpanded:  false,
			Fields: []PaymentField{
				{Name: "card_number", Label: "Card Number", Type: "text", Required: true, Placeholder: "1234 5678 9012 3456"},
				{Name: "expiry_date", Label: "Expiry Date", Type: "text", Required: true, Placeholder: "MM/YY"},
				{Name: "cvv", Label: "CVV", Type: "password", Required: true, Placeholder: "123"},
				{Name: "cardholder_name", Label: "Cardholder Name", Type: "text", Required: true, Placeholder: "John Doe"},
			},
		},
		{
			ID:          "pm_debit_card",
			Type:        PaymentMethodDebitCard,
			DisplayName: "Debit Card",
			Description: "Pay directly from your bank account",
			IconURL:     "/icons/debit_card.svg",
			IsExpanded:  false,
			Fields: []PaymentField{
				{Name: "card_number", Label: "Card Number", Type: "text", Required: true, Placeholder: "1234 5678 9012 3456"},
				{Name: "expiry_date", Label: "Expiry Date", Type: "text", Required: true, Placeholder: "MM/YY"},
				{Name: "cvv", Label: "CVV", Type: "password", Required: true, Placeholder: "123"},
				{Name: "cardholder_name", Label: "Cardholder Name", Type: "text", Required: true, Placeholder: "John Doe"},
			},
		},
		{
			ID:          "pm_internet_banking",
			Type:        PaymentMethodInternetBanking,
			DisplayName: "Internet Banking",
			Description: "Pay via your bank's internet banking portal",
			IconURL:     "/icons/internet_banking.svg",
			RedirectURL: "/payments/redirect/internet_banking",
			IsExpanded:  false,
			Fields: []PaymentField{
				{Name: "bank_name", Label: "Select Bank", Type: "select", Required: true},
			},
		},
		{
			ID:          "pm_banking_app",
			Type:        PaymentMethodBankingApp,
			DisplayName: "Banking App",
			Description: "Pay using your bank's mobile application",
			IconURL:     "/icons/banking_app.svg",
			RedirectURL: "/payments/redirect/banking_app",
			IsExpanded:  false,
			Fields: []PaymentField{
				{Name: "bank_name", Label: "Select Bank", Type: "select", Required: true},
			},
		},
		{
			ID:          "pm_paypal",
			Type:        PaymentMethodPayPal,
			DisplayName: "PayPal",
			Description: "Pay securely with your PayPal account",
			IconURL:     "/icons/paypal.svg",
			RedirectURL: "/payments/redirect/paypal",
			IsExpanded:  false,
			Fields:      []PaymentField{},
		},
	}
}

// findPaymentMethod looks up a payment method by its ID.
func findPaymentMethod(methodID string) *PaymentMethod {
	methods := getPaymentMethods()
	for i := range methods {
		if methods[i].ID == methodID {
			return &methods[i]
		}
	}
	return nil
}

// findPaymentMethodByType looks up a payment method by its type.
func findPaymentMethodByType(methodType PaymentMethodType) *PaymentMethod {
	methods := getPaymentMethods()
	for i := range methods {
		if methods[i].Type == methodType {
			return &methods[i]
		}
	}
	return nil
}

// generateRedirectURL builds the redirect URL for a given payment method type and order details.
func generateRedirectURL(methodType PaymentMethodType, orderID string, amount string, currency string, callbackURL string) (string, error) {
	switch methodType {
	case PaymentMethodInternetBanking:
		return fmt.Sprintf("/banking/authorize?order_id=%s&amount=%s&currency=%s&callback=%s",
			orderID, amount, currency, callbackURL), nil
	case PaymentMethodBankingApp:
		return fmt.Sprintf("/bankapp/launch?order_id=%s&amount=%s&currency=%s&callback=%s",
			orderID, amount, currency, callbackURL), nil
	case PaymentMethodPayPal:
		return fmt.Sprintf("https://www.paypal.com/checkout?order_id=%s&amount=%s&currency=%s&return=%s",
			orderID, amount, currency, callbackURL), nil
	default:
		return "", fmt.Errorf("payment method type '%s' does not support redirection", methodType)
	}
}

// jsonResponse is a helper that builds an API Gateway JSON response.
func jsonResponse(statusCode int, body interface{}) (events.APIGatewayProxyResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error":"failed to marshal response"}`,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       string(jsonBody),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

// errorResponse is a helper that builds an error JSON response.
func errorResponse(statusCode int, message string) (events.APIGatewayProxyResponse, error) {
	body := map[string]string{"error": message}
	return jsonResponse(statusCode, body)
}

// HandleListPaymentMethods handles GET /payments/methods - returns all available payment methods.
func HandleListPaymentMethods() (events.APIGatewayProxyResponse, error) {
	methods := getPaymentMethods()
	resp := PaymentMethodsResponse{
		PaymentMethods: methods,
		Count:          len(methods),
	}
	return jsonResponse(http.StatusOK, resp)
}

// HandleGetPaymentMethod handles GET /payments/methods/{id} - returns expanded details for a specific payment method.
func HandleGetPaymentMethod(methodID string) (events.APIGatewayProxyResponse, error) {
	if methodID == "" {
		return errorResponse(http.StatusBadRequest, "payment method ID is required")
	}

	method := findPaymentMethod(methodID)
	if method == nil {
		return errorResponse(http.StatusNotFound, fmt.Sprintf("payment method '%s' not found", methodID))
	}

	// Return expanded view
	method.IsExpanded = true
	return jsonResponse(http.StatusOK, method)
}

// HandlePaymentRedirect handles GET /payments/redirect/{type} - returns the redirect URL for a payment method.
func HandlePaymentRedirect(methodType string, queryParams map[string]string) (events.APIGatewayProxyResponse, error) {
	pmType := PaymentMethodType(methodType)

	method := findPaymentMethodByType(pmType)
	if method == nil {
		return errorResponse(http.StatusNotFound, fmt.Sprintf("payment method type '%s' not found", methodType))
	}

	// Validate that this method supports redirection
	if method.RedirectURL == "" {
		return errorResponse(http.StatusBadRequest, fmt.Sprintf("payment method '%s' does not support automatic redirection", methodType))
	}

	orderID := queryParams["order_id"]
	amount := queryParams["amount"]
	currency := queryParams["currency"]
	callbackURL := queryParams["callback_url"]

	if orderID == "" || amount == "" {
		return errorResponse(http.StatusBadRequest, "order_id and amount are required query parameters")
	}

	if currency == "" {
		currency = "USD"
	}

	if callbackURL == "" {
		callbackURL = "/payments/callback"
	}

	redirectURL, err := generateRedirectURL(pmType, orderID, amount, currency, callbackURL)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, err.Error())
	}

	resp := PaymentRedirectResponse{
		PaymentMethodType: pmType,
		RedirectURL:       redirectURL,
		SessionID:         fmt.Sprintf("sess_%s_%s", methodType, orderID),
	}

	return jsonResponse(http.StatusOK, resp)
}
