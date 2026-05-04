package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

type mockedPutItem struct {
	dynamodbiface.DynamoDBAPI
	Response dynamodb.PutItemOutput
	Err      error
}

func (m mockedPutItem) PutItem(input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	return &m.Response, m.Err
}

func newDependency(dynamoErr error) *Dependency {
	return &Dependency{
		DepDynamoDB: mockedPutItem{
			Response: dynamodb.PutItemOutput{},
			Err:      dynamoErr,
		},
	}
}

func TestListPaymentMethods(t *testing.T) {
	d := newDependency(nil)

	t.Run("List all payment methods", func(t *testing.T) {
		methods, err := d.listPaymentMethods("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(methods) == 0 {
			t.Fatal("expected at least one payment method")
		}
		// Verify all methods have required fields
		for _, m := range methods {
			if m.ID == "" {
				t.Error("payment method ID should not be empty")
			}
			if m.Name == "" {
				t.Error("payment method Name should not be empty")
			}
			if m.Icon == "" {
				t.Errorf("payment method %s should have an icon", m.ID)
			}
			if m.Type == "" {
				t.Errorf("payment method %s should have a type", m.ID)
			}
		}
	})

	t.Run("Filter by card type", func(t *testing.T) {
		methods, err := d.listPaymentMethods("card")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, m := range methods {
			if m.Type != TypeCard {
				t.Errorf("expected type card, got %s for method %s", m.Type, m.ID)
			}
		}
		if len(methods) == 0 {
			t.Error("expected at least one card payment method")
		}
	})

	t.Run("Filter by internet_banking type", func(t *testing.T) {
		methods, err := d.listPaymentMethods("internet_banking")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, m := range methods {
			if m.Type != TypeInternetBanking {
				t.Errorf("expected type internet_banking, got %s", m.Type)
			}
		}
		if len(methods) == 0 {
			t.Error("expected at least one internet banking method")
		}
	})

	t.Run("Filter by paypal type", func(t *testing.T) {
		methods, err := d.listPaymentMethods("paypal")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(methods) != 1 {
			t.Fatalf("expected 1 paypal method, got %d", len(methods))
		}
		if methods[0].ID != "pm_paypal" {
			t.Errorf("expected pm_paypal, got %s", methods[0].ID)
		}
	})

	t.Run("Filter by nonexistent type returns empty", func(t *testing.T) {
		methods, err := d.listPaymentMethods("crypto")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(methods) != 0 {
			t.Errorf("expected 0 methods, got %d", len(methods))
		}
	})
}

func TestGetPaymentMethodByID(t *testing.T) {
	d := newDependency(nil)

	t.Run("Existing method", func(t *testing.T) {
		method, err := d.getPaymentMethodByID("pm_visa")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if method.Name != "Visa" {
			t.Errorf("expected Visa, got %s", method.Name)
		}
		if method.Icon == "" {
			t.Error("expected icon to be set")
		}
	})

	t.Run("Non-existing method", func(t *testing.T) {
		_, err := d.getPaymentMethodByID("pm_nonexistent")
		if err == nil {
			t.Error("expected error for non-existing method")
		}
	})
}

func TestGetExpandedView(t *testing.T) {
	d := newDependency(nil)

	t.Run("Expandable method returns details", func(t *testing.T) {
		method, err := d.getExpandedView("pm_visa")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if method.ExpandedDetails == nil {
			t.Fatal("expected expanded details")
		}
		if len(method.ExpandedDetails.Fields) == 0 {
			t.Error("expected fields in expanded details")
		}
	})

	t.Run("Internet banking expanded view has supported banks", func(t *testing.T) {
		method, err := d.getExpandedView("pm_internet_banking")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if method.ExpandedDetails == nil {
			t.Fatal("expected expanded details")
		}
		if len(method.ExpandedDetails.SupportedBanks) == 0 {
			t.Error("expected supported banks in expanded details")
		}
		if method.ExpandedDetails.Instructions == "" {
			t.Error("expected instructions in expanded details")
		}
	})

	t.Run("Non-existing method returns error", func(t *testing.T) {
		_, err := d.getExpandedView("pm_nonexistent")
		if err == nil {
			t.Error("expected error for non-existing method")
		}
	})
}

func TestGetRedirectURL(t *testing.T) {
	d := newDependency(nil)

	t.Run("Internet banking redirect", func(t *testing.T) {
		redirect, err := d.getRedirectURL("pm_internet_banking")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if redirect.RedirectURL == "" {
			t.Error("expected redirect URL")
		}
		if redirect.PaymentMethodID != "pm_internet_banking" {
			t.Errorf("expected pm_internet_banking, got %s", redirect.PaymentMethodID)
		}
		if redirect.ExpiresIn <= 0 {
			t.Error("expected positive expiry time")
		}
	})

	t.Run("Third party banking app redirect", func(t *testing.T) {
		redirect, err := d.getRedirectURL("pm_third_party_banking")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if redirect.RedirectURL == "" {
			t.Error("expected redirect URL for third party banking")
		}
	})

	t.Run("PayPal redirect", func(t *testing.T) {
		redirect, err := d.getRedirectURL("pm_paypal")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if redirect.RedirectURL == "" {
			t.Error("expected redirect URL for PayPal")
		}
	})

	t.Run("Card method has no redirect", func(t *testing.T) {
		_, err := d.getRedirectURL("pm_visa")
		if err == nil {
			t.Error("expected error for card method without redirect")
		}
	})

	t.Run("Non-existing method returns error", func(t *testing.T) {
		_, err := d.getRedirectURL("pm_nonexistent")
		if err == nil {
			t.Error("expected error for non-existing method")
		}
	})
}

func TestGetPaymentMethodIcon(t *testing.T) {
	t.Run("Get icon for existing method", func(t *testing.T) {
		icon, err := GetPaymentMethodIcon("pm_visa")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if icon == "" {
			t.Error("expected icon URL")
		}
	})

	t.Run("Get icon for PayPal", func(t *testing.T) {
		icon, err := GetPaymentMethodIcon("pm_paypal")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if icon == "" {
			t.Error("expected icon URL for PayPal")
		}
	})

	t.Run("Non-existing method returns error", func(t *testing.T) {
		_, err := GetPaymentMethodIcon("pm_nonexistent")
		if err == nil {
			t.Error("expected error for non-existing method")
		}
	})
}

func TestIsValidPaymentMethodType(t *testing.T) {
	validTypes := []string{"card", "internet_banking", "third_party_app", "paypal", "wallet"}
	for _, vt := range validTypes {
		if !isValidPaymentMethodType(vt) {
			t.Errorf("expected %s to be valid", vt)
		}
	}

	invalidTypes := []string{"crypto", "cash", "", "unknown"}
	for _, it := range invalidTypes {
		if isValidPaymentMethodType(it) {
			t.Errorf("expected %s to be invalid", it)
		}
	}
}

func TestExtractMethodID(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/payment/methods/pm_visa", "pm_visa"},
		{"/payment/methods/pm_visa/expand", "pm_visa"},
		{"/payment/methods/pm_visa/redirect", "pm_visa"},
		{"/payment/methods/pm_visa/select", "pm_visa"},
		{"/Prod/payment/methods/pm_visa", "pm_visa"},
		{"/Prod/payment/methods/pm_visa/expand", "pm_visa"},
		{"/payment/methods", ""},
		{"/invalid/path", ""},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("path=%s", tc.path), func(t *testing.T) {
			result := extractMethodID(tc.path)
			if result != tc.expected {
				t.Errorf("extractMethodID(%q) = %q, want %q", tc.path, result, tc.expected)
			}
		})
	}
}

func TestHandlerListMethods(t *testing.T) {
	d := newDependency(nil)
	ctx := context.Background()

	t.Run("GET /payment/methods returns all methods", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods",
			HTTPMethod: "GET",
			QueryStringParameters: map[string]string{},
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
		}

		var body PaymentMethodsResponse
		if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if body.Total == 0 {
			t.Error("expected payment methods in response")
		}
		if len(body.PaymentMethods) != body.Total {
			t.Errorf("total mismatch: methods=%d, total=%d", len(body.PaymentMethods), body.Total)
		}

		// Verify all methods have icons
		for _, m := range body.PaymentMethods {
			if m.Icon == "" {
				t.Errorf("method %s missing icon", m.ID)
			}
		}
	})

	t.Run("GET /payment/methods?type=card returns only cards", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods",
			HTTPMethod: "GET",
			QueryStringParameters: map[string]string{"type": "card"},
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var body PaymentMethodsResponse
		json.Unmarshal([]byte(resp.Body), &body)
		for _, m := range body.PaymentMethods {
			if m.Type != TypeCard {
				t.Errorf("expected card type, got %s", m.Type)
			}
		}
	})
}

func TestHandlerGetMethod(t *testing.T) {
	d := newDependency(nil)
	ctx := context.Background()

	t.Run("GET /payment/methods/pm_visa returns method", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods/pm_visa",
			HTTPMethod: "GET",
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
		}

		var method PaymentMethod
		json.Unmarshal([]byte(resp.Body), &method)
		if method.ID != "pm_visa" {
			t.Errorf("expected pm_visa, got %s", method.ID)
		}
		if method.Icon == "" {
			t.Error("expected icon URL")
		}
	})

	t.Run("GET /payment/methods/pm_nonexistent returns 404", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods/pm_nonexistent",
			HTTPMethod: "GET",
		}
		resp, _ := d.Handler(ctx, request)
		if resp.StatusCode != 404 {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}

func TestHandlerExpandView(t *testing.T) {
	d := newDependency(nil)
	ctx := context.Background()

	t.Run("GET /payment/methods/pm_visa/expand returns expanded details", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods/pm_visa/expand",
			HTTPMethod: "GET",
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
		}

		var method PaymentMethod
		json.Unmarshal([]byte(resp.Body), &method)
		if method.ExpandedDetails == nil {
			t.Error("expected expanded details in response")
		}
	})

	t.Run("GET /payment/methods/pm_internet_banking/expand shows bank info", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods/pm_internet_banking/expand",
			HTTPMethod: "GET",
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var method PaymentMethod
		json.Unmarshal([]byte(resp.Body), &method)
		if method.ExpandedDetails == nil || len(method.ExpandedDetails.SupportedBanks) == 0 {
			t.Error("expected supported banks in expanded details")
		}
	})
}

func TestHandlerRedirect(t *testing.T) {
	d := newDependency(nil)
	ctx := context.Background()

	t.Run("GET /payment/methods/pm_internet_banking/redirect returns redirect URL", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods/pm_internet_banking/redirect",
			HTTPMethod: "GET",
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
		}

		var redirect RedirectResponse
		json.Unmarshal([]byte(resp.Body), &redirect)
		if redirect.RedirectURL == "" {
			t.Error("expected redirect URL")
		}
	})

	t.Run("GET /payment/methods/pm_paypal/redirect returns PayPal redirect", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods/pm_paypal/redirect",
			HTTPMethod: "GET",
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var redirect RedirectResponse
		json.Unmarshal([]byte(resp.Body), &redirect)
		if redirect.RedirectURL == "" {
			t.Error("expected PayPal redirect URL")
		}
	})

	t.Run("GET /payment/methods/pm_third_party_banking/redirect returns banking app redirect", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods/pm_third_party_banking/redirect",
			HTTPMethod: "GET",
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("GET /payment/methods/pm_visa/redirect returns error for card", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods/pm_visa/redirect",
			HTTPMethod: "GET",
		}
		resp, _ := d.Handler(ctx, request)
		if resp.StatusCode != 400 {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})
}

func TestHandlerSelectMethod(t *testing.T) {
	d := newDependency(nil)
	ctx := context.Background()

	t.Run("POST /payment/methods/pm_visa/select records selection", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods/pm_visa/select",
			HTTPMethod: "POST",
			Body:       `{"user_id": "user_123"}`,
		}
		resp, err := d.Handler(ctx, request)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
		}
	})

	t.Run("POST /payment/methods/pm_visa/select without user_id returns 400", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods/pm_visa/select",
			HTTPMethod: "POST",
			Body:       `{}`,
		}
		resp, _ := d.Handler(ctx, request)
		if resp.StatusCode != 400 {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("POST /payment/methods/pm_nonexistent/select returns 404", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/payment/methods/pm_nonexistent/select",
			HTTPMethod: "POST",
			Body:       `{"user_id": "user_123"}`,
		}
		resp, _ := d.Handler(ctx, request)
		if resp.StatusCode != 404 {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}

func TestHandlerInvalidPaths(t *testing.T) {
	d := newDependency(nil)
	ctx := context.Background()

	t.Run("Invalid path returns error", func(t *testing.T) {
		request := events.APIGatewayProxyRequest{
			Path:       "/invalid/path",
			HTTPMethod: "GET",
		}
		resp, _ := d.Handler(ctx, request)
		if resp.StatusCode != 400 {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})
}

func TestDiversePaymentMethodTypes(t *testing.T) {
	d := newDependency(nil)
	methods, _ := d.listPaymentMethods("")

	typeSet := make(map[PaymentMethodType]bool)
	for _, m := range methods {
		typeSet[m.Type] = true
	}

	expectedTypes := []PaymentMethodType{TypeCard, TypeInternetBanking, TypeThirdPartyApp, TypePayPal, TypeWallet}
	for _, et := range expectedTypes {
		if !typeSet[et] {
			t.Errorf("expected payment method type %s to be present", et)
		}
	}
}

func TestAllMethodsHaveIcons(t *testing.T) {
	d := newDependency(nil)
	methods, _ := d.listPaymentMethods("")

	for _, m := range methods {
		if m.Icon == "" {
			t.Errorf("payment method %s (%s) is missing an icon", m.Name, m.ID)
		}
	}
}

func TestRedirectMethodsExist(t *testing.T) {
	d := newDependency(nil)

	// These methods must support redirection per the ticket requirements
	redirectMethods := []string{"pm_internet_banking", "pm_third_party_banking", "pm_paypal"}
	for _, id := range redirectMethods {
		redirect, err := d.getRedirectURL(id)
		if err != nil {
			t.Errorf("expected %s to support redirection, got error: %v", id, err)
			continue
		}
		if redirect.RedirectURL == "" {
			t.Errorf("expected %s to have a redirect URL", id)
		}
	}
}
