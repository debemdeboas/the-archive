package auth

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/debemdeboas/the-archive/internal/auth/testdata"
)

func TestEd25519ChallengeHandler(t *testing.T) {
	provider, err := NewEd25519AuthProvider(testdata.TestPublicKeyPEM, "Authorization", testdata.TestUserID)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	handler := Ed25519ChallengeHandler(provider)

	testCases := []struct {
		name           string
		method         string
		expectedStatus int
		expectJSON     bool
		expectNewChallenge bool
	}{
		{
			name:           "GET challenge returns a new challenge",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectJSON:     true,
			expectNewChallenge: true,
		},
		{
			name:           "POST challenge generates new challenge",
			method:         http.MethodPost,
			expectedStatus: http.StatusOK,
			expectJSON:     true,
			expectNewChallenge: true,
		},
		{
			name:           "PUT method not allowed",
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
			expectJSON:     false,
			expectNewChallenge: false,
		},
		{
			name:           "DELETE method not allowed",
			method:         http.MethodDelete,
			expectedStatus: http.StatusMethodNotAllowed,
			expectJSON:     false,
			expectNewChallenge: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Store original challenge to compare
			originalChallenge := make([]byte, len(provider.challenge))
			copy(originalChallenge, provider.challenge)

			req := httptest.NewRequest(tc.method, "/auth/challenge", nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			if recorder.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, recorder.Code)
			}

			if tc.expectJSON {
				contentType := recorder.Header().Get("Content-Type")
				if !strings.Contains(contentType, "application/json") {
					t.Errorf("Expected JSON content type, got %s", contentType)
				}

				var response map[string]string
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				if err != nil {
					t.Errorf("Failed to parse JSON response: %v", err)
				}

				challenge, exists := response["challenge"]
				if !exists {
					t.Error("Expected 'challenge' field in response")
				}

				// Verify challenge is valid base64
				_, err = base64.StdEncoding.DecodeString(challenge)
				if err != nil {
					t.Errorf("Challenge is not valid base64: %v", err)
				}

				// Check if challenge changed when expected
				newChallenge := provider.GetChallenge()
				challengeChanged := !strings.EqualFold(string(originalChallenge), string(newChallenge))
				
				if tc.expectNewChallenge && !challengeChanged {
					t.Error("Expected challenge to change, but it didn't")
				}
				if !tc.expectNewChallenge && challengeChanged {
					t.Error("Expected challenge to stay the same, but it changed")
				}
			}
		})
	}
}

func TestEd25519VerifyHandler(t *testing.T) {
	provider, err := NewEd25519AuthProvider(testdata.TestPublicKeyPEM, "Authorization", testdata.TestUserID)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Set a fixed challenge for consistent testing
	provider.challenge = testdata.TestChallenge

	handler := Ed25519VerifyHandler(provider)

	// Generate a valid signature for testing
	validSignature := generateValidSignature(t, provider.challenge)
	validSignatureB64 := base64.StdEncoding.EncodeToString(validSignature)

	testCases := []struct {
		name           string
		method         string
		authHeader     string
		expectedStatus int
		expectCookie   bool
		tlsRequest     bool
	}{
		{
			name:           "Valid signature verification",
			method:         http.MethodPost,
			authHeader:     validSignatureB64,
			expectedStatus: http.StatusOK,
			expectCookie:   true,
			tlsRequest:     false,
		},
		{
			name:           "Valid signature with TLS - secure cookie",
			method:         http.MethodPost,
			authHeader:     validSignatureB64,
			expectedStatus: http.StatusOK,
			expectCookie:   true,
			tlsRequest:     true,
		},
		{
			name:           "Invalid signature",
			method:         http.MethodPost,
			authHeader:     "invalid-signature-base64",
			expectedStatus: http.StatusUnauthorized,
			expectCookie:   false,
		},
		{
			name:           "Missing authorization header",
			method:         http.MethodPost,
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectCookie:   false,
		},
		{
			name:           "Invalid base64 signature",
			method:         http.MethodPost,
			authHeader:     "not-valid-base64!@#",
			expectedStatus: http.StatusUnauthorized,
			expectCookie:   false,
		},
		{
			name:           "GET method not allowed",
			method:         http.MethodGet,
			authHeader:     validSignatureB64,
			expectedStatus: http.StatusMethodNotAllowed,
			expectCookie:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/auth/verify", nil)
			
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			// Mock TLS if needed
			if tc.tlsRequest {
				req.TLS = &tls.ConnectionState{}
			}

			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			if recorder.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, recorder.Code)
			}

			// Check cookie expectations
			cookies := recorder.Result().Cookies()
			foundAuthCookie := false
			for _, cookie := range cookies {
				if cookie.Name == "auth_token" {
					foundAuthCookie = true

					// Verify cookie properties
					if cookie.Path != "/" {
						t.Errorf("Expected cookie path '/', got '%s'", cookie.Path)
					}
					if !cookie.HttpOnly {
						t.Error("Expected cookie to be HttpOnly")
					}
					if cookie.SameSite != http.SameSiteStrictMode {
						t.Errorf("Expected SameSite strict mode, got %v", cookie.SameSite)
					}
					if tc.tlsRequest && !cookie.Secure {
						t.Error("Expected secure cookie for TLS request")
					}
					if !tc.tlsRequest && cookie.Secure {
						t.Error("Did not expect secure cookie for non-TLS request")
					}
					if cookie.MaxAge != 3600*24 {
						t.Errorf("Expected MaxAge 86400, got %d", cookie.MaxAge)
					}

					// Verify cookie value is the signature
					if cookie.Value != tc.authHeader {
						t.Errorf("Expected cookie value '%s', got '%s'", tc.authHeader, cookie.Value)
					}
					break
				}
			}

			if tc.expectCookie && !foundAuthCookie {
				t.Error("Expected auth_token cookie, but didn't find one")
			}
			if !tc.expectCookie && foundAuthCookie {
				t.Error("Did not expect auth_token cookie, but found one")
			}
		})
	}
}

func TestEd25519AuthPageHandler(t *testing.T) {
	provider, err := NewEd25519AuthProvider(testdata.TestPublicKeyPEM, "Authorization", testdata.TestUserID)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create a minimal template for testing
	tmplContent := `{{define "ed25519_auth"}}
<html>
<head><title>Auth Page</title></head>
<body>
<p>Redirect URL: {{.RedirectURL}}</p>
</body>
</html>
{{end}}`

	tmpl, err := template.New("test").Parse(tmplContent)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	handler := Ed25519AuthPageHandler(provider, tmpl)

	testCases := []struct {
		name                string
		queryParams         string
		expectedStatus      int
		expectedContentType string
		expectRedirectURL   string
		expectHxRedirect    string
	}{
		{
			name:                "Default redirect to root",
			queryParams:         "",
			expectedStatus:      http.StatusOK,
			expectedContentType: "text/html",
			expectRedirectURL:   "/",
			expectHxRedirect:    "/",
		},
		{
			name:                "Custom redirect URL",
			queryParams:         "?redirect=/dashboard",
			expectedStatus:      http.StatusOK,
			expectedContentType: "text/html",
			expectRedirectURL:   "/dashboard",
			expectHxRedirect:    "/dashboard",
		},
		{
			name:                "Refresh parameter triggers redirect to login",
			queryParams:         "?refresh=true",
			expectedStatus:      http.StatusOK,
			expectedContentType: "text/html",
			expectRedirectURL:   "/",
			expectHxRedirect:    "/auth/login",
		},
		{
			name:                "Refresh false doesn't trigger redirect",
			queryParams:         "?refresh=false&redirect=/custom",
			expectedStatus:      http.StatusOK,
			expectedContentType: "text/html",
			expectRedirectURL:   "/custom",
			expectHxRedirect:    "/custom",
		},
		{
			name:                "Both redirect and refresh parameters",
			queryParams:         "?redirect=/admin&refresh=true",
			expectedStatus:      http.StatusOK,
			expectedContentType: "text/html",
			expectRedirectURL:   "/admin",
			expectHxRedirect:    "/auth/login", // refresh overrides redirect for Hx-Redirect
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/auth/login"+tc.queryParams, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			if recorder.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, recorder.Code)
			}

			contentType := recorder.Header().Get("Content-Type")
			if !strings.Contains(contentType, tc.expectedContentType) {
				t.Errorf("Expected content type to contain '%s', got '%s'", tc.expectedContentType, contentType)
			}

			hxRedirect := recorder.Header().Get("Hx-Redirect")
			if hxRedirect != tc.expectHxRedirect {
				t.Errorf("Expected Hx-Redirect '%s', got '%s'", tc.expectHxRedirect, hxRedirect)
			}

			// Check that the redirect URL appears in the response body
			body := recorder.Body.String()
			if !strings.Contains(body, tc.expectRedirectURL) {
				t.Errorf("Expected response body to contain redirect URL '%s', body: %s", tc.expectRedirectURL, body)
			}
		})
	}
}

func TestEd25519AuthPageHandler_TemplateError(t *testing.T) {
	provider, err := NewEd25519AuthProvider(testdata.TestPublicKeyPEM, "Authorization", testdata.TestUserID)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create a template that will fail to execute
	tmplContent := `{{define "ed25519_auth"}}{{.NonExistentField.BadCall}}{{end}}`
	tmpl, err := template.New("test").Parse(tmplContent)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	handler := Ed25519AuthPageHandler(provider, tmpl)

	req := httptest.NewRequest("GET", "/auth/login", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d for template error, got %d", http.StatusInternalServerError, recorder.Code)
	}
}

