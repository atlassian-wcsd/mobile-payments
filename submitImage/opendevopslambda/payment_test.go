package opendevopslambda

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
)

func TestHandleListPaymentMethods(t *testing.T) {
	t.Run("Returns all payment methods", func(t *testing.T) {
		resp, err := HandleListPaymentMethods()
		if err != nil {
			t.Fatalf("HandleListPaymentMethods returned error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var result PaymentMethodsResponse
		if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if result.Count != 5 {
			t.Errorf("expected 5 payment methods, got %d", result.Count)
		}

		if len(result.PaymentMethods) != 5 {
			t.Fatalf("expected 5 payment methods in array, got %d", len(result.PaymentMethods))
		}

		// Verify each payment method has required fields
		for _, pm := range result.PaymentMethods {
			if pm.ID == "" {
				t.Error("payment method ID is empty")
			}
			if pm.DisplayName == "" {
				t.Error("payment method display name is empty")
			}
			if pm.IconURL == "" {
				t.Errorf("payment method %s is missing icon URL", pm.ID)
			}
			if pm.Description == "" {
				t.Errorf("payment method %s is missing description", pm.ID)
			}
		}
	})

	t.Run("All payment methods have icons", func(t *testing.T) {
		resp, _ := HandleListPaymentMethods()
		var result PaymentMethodsResponse
		json.Unmarshal([]byte(resp.Body), &result)

		expectedIcons := map[PaymentMethodType]string{
			PaymentMethodCreditCard:      "/icons/credit_card.svg",
			PaymentMethodDebitCard:        "/icons/debit_card.svg",
			PaymentMethodInternetBanking:  "/icons/internet_banking.svg",
			PaymentMethodBankingApp:       "/icons/banking_app.svg",
			PaymentMethodPayPal:           "/icons/paypal.svg",
		}

		for _, pm := range result.PaymentMethods {
			expectedIcon, exists := expectedIcons[pm.Type]
			if !exists {
				t.Errorf("unexpected payment method type: %s", pm.Type)
				continue
			}
			if pm.IconURL != expectedIcon {
				t.Errorf("payment method %s: expected icon %s, got %s", pm.Type, expectedIcon, pm.IconURL)
			}
		}
	})

	t.Run("Redirect payment methods have redirect URLs", func(t *testing.T) {
		resp, _ := HandleListPaymentMethods()
		var result PaymentMethodsResponse
		json.Unmarshal([]byte(resp.Body), &result)

		redirectTypes := map[PaymentMethodType]bool{
			PaymentMethodInternetBanking: true,
			PaymentMethodBankingApp:      true,
			PaymentMethodPayPal:          true,
		}

		for _, pm := range result.PaymentMethods {
			if redirectTypes[pm.Type] {
				if pm.RedirectURL == "" {
					t.Errorf("payment method %s should have a redirect URL", pm.Type)
				}
			}
		}
	})
}

func TestHandleGetPaymentMethod(t *testing.T) {
	t.Run("Valid payment method ID returns expanded view", func(t *testing.T) {
		resp, err := HandleGetPaymentMethod("pm_credit_card")
		if err != nil {
			t.Fatalf("HandleGetPaymentMethod returned error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var pm PaymentMethod
		if err := json.Unmarshal([]byte(resp.Body), &pm); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if !pm.IsExpanded {
			t.Error("expected IsExpanded to be true")
		}
		if pm.ID != "pm_credit_card" {
			t.Errorf("expected ID pm_credit_card, got %s", pm.ID)
		}
		if len(pm.Fields) == 0 {
			t.Error("expected fields in expanded view")
		}
	})

	t.Run("Expanded credit card has correct fields", func(t *testing.T) {
		resp, _ := HandleGetPaymentMethod("pm_credit_card")
		var pm PaymentMethod
		json.Unmarshal([]byte(resp.Body), &pm)

		expectedFields := []string{"card_number", "expiry_date", "cvv", "cardholder_name"}
		if len(pm.Fields) != len(expectedFields) {
			t.Fatalf("expected %d fields, got %d", len(expectedFields), len(pm.Fields))
		}
		for i, field := range pm.Fields {
			if field.Name != expectedFields[i] {
				t.Errorf("field %d: expected name %s, got %s", i, expectedFields[i], field.Name)
			}
			if !field.Required {
				t.Errorf("field %s should be required", field.Name)
			}
		}
	})

	t.Run("Invalid payment method ID returns 404", func(t *testing.T) {
		resp, err := HandleGetPaymentMethod("pm_nonexistent")
		if err != nil {
			t.Fatalf("HandleGetPaymentMethod returned error: %v", err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("Empty payment method ID returns 400", func(t *testing.T) {
		resp, err := HandleGetPaymentMethod("")
		if err != nil {
			t.Fatalf("HandleGetPaymentMethod returned error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})
}

func TestHandlePaymentRedirect(t *testing.T) {
	t.Run("Internet banking redirect", func(t *testing.T) {
		params := map[string]string{
			"order_id": "ORD-123",
			"amount":   "99.99",
			"currency": "USD",
		}
		resp, err := HandlePaymentRedirect("internet_banking", params)
		if err != nil {
			t.Fatalf("HandlePaymentRedirect returned error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", resp.StatusCode, resp.Body)
		}

		var result PaymentRedirectResponse
		if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if result.RedirectURL == "" {
			t.Error("expected redirect URL to be non-empty")
		}
		if result.PaymentMethodType != PaymentMethodInternetBanking {
			t.Errorf("expected payment method type internet_banking, got %s", result.PaymentMethodType)
		}
	})

	t.Run("Banking app redirect", func(t *testing.T) {
		params := map[string]string{
			"order_id": "ORD-456",
			"amount":   "50.00",
		}
		resp, err := HandlePaymentRedirect("banking_app", params)
		if err != nil {
			t.Fatalf("HandlePaymentRedirect returned error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var result PaymentRedirectResponse
		json.Unmarshal([]byte(resp.Body), &result)

		if result.PaymentMethodType != PaymentMethodBankingApp {
			t.Errorf("expected payment method type banking_app, got %s", result.PaymentMethodType)
		}
	})

	t.Run("PayPal redirect", func(t *testing.T) {
		params := map[string]string{
			"order_id":     "ORD-789",
			"amount":       "25.00",
			"currency":     "EUR",
			"callback_url": "https://example.com/callback",
		}
		resp, err := HandlePaymentRedirect("paypal", params)
		if err != nil {
			t.Fatalf("HandlePaymentRedirect returned error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var result PaymentRedirectResponse
		json.Unmarshal([]byte(resp.Body), &result)

		if result.PaymentMethodType != PaymentMethodPayPal {
			t.Errorf("expected payment method type paypal, got %s", result.PaymentMethodType)
		}
		if result.RedirectURL == "" {
			t.Error("expected redirect URL")
		}
	})

	t.Run("Default currency is USD", func(t *testing.T) {
		params := map[string]string{
			"order_id": "ORD-100",
			"amount":   "10.00",
		}
		resp, _ := HandlePaymentRedirect("internet_banking", params)
		var result PaymentRedirectResponse
		json.Unmarshal([]byte(resp.Body), &result)

		if result.RedirectURL == "" {
			t.Error("expected redirect URL")
		}
	})

	t.Run("Credit card does not support redirect", func(t *testing.T) {
		params := map[string]string{
			"order_id": "ORD-200",
			"amount":   "10.00",
		}
		resp, err := HandlePaymentRedirect("credit_card", params)
		if err != nil {
			t.Fatalf("HandlePaymentRedirect returned error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("Invalid payment method type returns 404", func(t *testing.T) {
		params := map[string]string{
			"order_id": "ORD-300",
			"amount":   "10.00",
		}
		resp, err := HandlePaymentRedirect("bitcoin", params)
		if err != nil {
			t.Fatalf("HandlePaymentRedirect returned error: %v", err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("Missing required params returns 400", func(t *testing.T) {
		params := map[string]string{}
		resp, err := HandlePaymentRedirect("paypal", params)
		if err != nil {
			t.Fatalf("HandlePaymentRedirect returned error: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})
}

func TestPaymentRouting(t *testing.T) {
	mpo := mockedPutOjbect{
		Response: s3.PutObjectOutput{},
	}
	mpi := mockedPutItem{
		Response: dynamodb.PutItemOutput{},
	}
	d := Dependency{
		DepS3:       mpo,
		DepDynamoDB: mpi,
	}

	ctx := context.Background()
	lc := new(lambdacontext.LambdaContext)
	lc.InvokedFunctionArn = "arn:aws:lambda:region:123456789000:function:functionName"
	ctx = lambdacontext.NewContext(ctx, lc)

	t.Run("Route to list payment methods", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payments/methods",
			HTTPMethod: "GET",
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var result PaymentMethodsResponse
		json.Unmarshal([]byte(resp.Body), &result)
		if result.Count != 5 {
			t.Errorf("expected 5 payment methods, got %d", result.Count)
		}
	})

	t.Run("Route to get specific payment method", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payments/methods/pm_paypal",
			HTTPMethod: "GET",
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var pm PaymentMethod
		json.Unmarshal([]byte(resp.Body), &pm)
		if pm.ID != "pm_paypal" {
			t.Errorf("expected pm_paypal, got %s", pm.ID)
		}
		if !pm.IsExpanded {
			t.Error("expected expanded view")
		}
	})

	t.Run("Route to payment redirect", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payments/redirect/paypal",
			HTTPMethod: "GET",
			QueryStringParameters: map[string]string{
				"order_id": "ORD-001",
				"amount":   "50.00",
			},
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Route with Prod prefix", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/Prod/payments/methods",
			HTTPMethod: "GET",
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Original image handler still works", func(t *testing.T) {
		qsp := map[string]string{}
		qsp["url"] = "https://cdn.britannica.com/05/30105-004-644BE36D.jpg"

		request := events.APIGatewayProxyRequest{
			Path:                  "/bootstrap",
			HTTPMethod:            "GET",
			QueryStringParameters: qsp,
		}

		_, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatal(fmt.Sprintf("Original handler failed with %s", err.Error()))
		}
	})
}

func TestGenerateRedirectURL(t *testing.T) {
	t.Run("Internet banking URL format", func(t *testing.T) {
		url, err := generateRedirectURL(PaymentMethodInternetBanking, "ORD-1", "100.00", "USD", "/callback")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url == "" {
			t.Error("expected non-empty URL")
		}
	})

	t.Run("Banking app URL format", func(t *testing.T) {
		url, err := generateRedirectURL(PaymentMethodBankingApp, "ORD-2", "200.00", "GBP", "/callback")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url == "" {
			t.Error("expected non-empty URL")
		}
	})

	t.Run("PayPal URL format", func(t *testing.T) {
		url, err := generateRedirectURL(PaymentMethodPayPal, "ORD-3", "50.00", "EUR", "https://example.com/return")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url == "" {
			t.Error("expected non-empty URL")
		}
	})

	t.Run("Unsupported type returns error", func(t *testing.T) {
		_, err := generateRedirectURL(PaymentMethodCreditCard, "ORD-4", "10.00", "USD", "/callback")
		if err == nil {
			t.Error("expected error for unsupported payment method type")
		}
	})
}

func TestFindPaymentMethod(t *testing.T) {
	t.Run("Find existing method by ID", func(t *testing.T) {
		pm := findPaymentMethod("pm_paypal")
		if pm == nil {
			t.Fatal("expected to find pm_paypal")
		}
		if pm.Type != PaymentMethodPayPal {
			t.Errorf("expected type paypal, got %s", pm.Type)
		}
	})

	t.Run("Find non-existing method returns nil", func(t *testing.T) {
		pm := findPaymentMethod("pm_nonexistent")
		if pm != nil {
			t.Error("expected nil for non-existing payment method")
		}
	})
}

func TestFindPaymentMethodByType(t *testing.T) {
	t.Run("Find existing method by type", func(t *testing.T) {
		pm := findPaymentMethodByType(PaymentMethodInternetBanking)
		if pm == nil {
			t.Fatal("expected to find internet_banking")
		}
		if pm.ID != "pm_internet_banking" {
			t.Errorf("expected ID pm_internet_banking, got %s", pm.ID)
		}
	})

	t.Run("Find non-existing type returns nil", func(t *testing.T) {
		pm := findPaymentMethodByType("bitcoin")
		if pm != nil {
			t.Error("expected nil for non-existing payment method type")
		}
	})
}
