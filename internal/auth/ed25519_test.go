package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/debemdeboas/the-archive/internal/auth/testdata"
	"github.com/debemdeboas/the-archive/internal/model"
)

const errUnexpected = "Unexpected error: %v"
const errExpectedErrorGotNone = "Expected error but got none"
const errExpectedUserIDGotAnother = "Expected user ID '%s', got '%s'"

const failedToCreateProvider = "Failed to create provider: %v"

func TestNewEd25519AuthProvider(t *testing.T) {
	testCases := []struct {
		name        string
		publicKey   string
		headerName  string
		userID      model.UserID
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid public key",
			publicKey:   testdata.TestPublicKeyPEM,
			headerName:  "Authorization",
			userID:      testdata.TestUserID,
			expectError: false,
		},
		{
			name:        "Invalid PEM format",
			publicKey:   "invalid-pem-data",
			headerName:  "Authorization",
			userID:      testdata.TestUserID,
			expectError: true,
			errorMsg:    "failed to parse PEM block containing the public key",
		},
		{
			name: "Valid PEM but not Ed25519",
			publicKey: `-----BEGIN PUBLIC KEY-----
MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAK3H5Q9+6YHl8/2V2yc7Kc1XvZKp4Fsr
X5g7H8Y9V2sF8b3p1LZN4h6f8e9X4D7B5Z0P4p2nF8h7gY3e2Q5k8Z0CAwEAAQ==
-----END PUBLIC KEY-----`,
			headerName:  "Authorization",
			userID:      testdata.TestUserID,
			expectError: true,
			errorMsg:    "key is not an Ed25519 public key",
		},
		{
			name:        "Empty header name should work",
			publicKey:   testdata.TestPublicKeyPEM,
			headerName:  "",
			userID:      testdata.TestUserID,
			expectError: false,
		},
		{
			name:        "Empty user ID should work",
			publicKey:   testdata.TestPublicKeyPEM,
			headerName:  "Authorization",
			userID:      "",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := NewEd25519AuthProvider(tc.publicKey, tc.headerName, tc.userID)

			if tc.expectError {
				if err == nil {
					t.Errorf(errExpectedErrorGotNone)
				} else if tc.errorMsg != "" && err.Error() != tc.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tc.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf(errUnexpected, err)
				return
			}

			if provider == nil {
				t.Fatal("Expected provider to be non-nil")
			}

			if provider.headerName != tc.headerName {
				t.Errorf("Expected header name '%s', got '%s'", tc.headerName, provider.headerName)
			}

			if provider.userID != tc.userID {
				t.Errorf(errExpectedUserIDGotAnother, tc.userID, provider.userID)
			}

			if provider.cookieName != "auth_token" {
				t.Errorf("Expected cookie name 'auth_token', got '%s'", provider.cookieName)
			}

			if len(provider.challenge) != 32 {
				t.Errorf("Expected challenge length 32, got %d", len(provider.challenge))
			}

			if provider.publicKey == nil {
				t.Error("Expected public key to be set")
			}
		})
	}
}

func TestEd25519AuthProvider_WithHeaderAuthorization(t *testing.T) {
	provider, err := NewEd25519AuthProvider(testdata.TestPublicKeyPEM, "Authorization", testdata.TestUserID)
	if err != nil {
		t.Fatalf(failedToCreateProvider, err)
	}

	// Set a fixed challenge for consistent testing
	provider.challenge = testdata.TestChallenge

	// Generate a valid signature for testing
	validSignature := generateValidSignature(t, provider.challenge)

	testCases := []struct {
		name           string
		setupRequest   func(*http.Request)
		expectUserID   bool
		expectedUserID model.UserID
	}{
		{
			name: "Valid signature in header",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", base64.StdEncoding.EncodeToString(validSignature))
			},
			expectUserID:   true,
			expectedUserID: testdata.TestUserID,
		},
		{
			name: "Valid signature in cookie",
			setupRequest: func(r *http.Request) {
				r.AddCookie(&http.Cookie{
					Name:  "auth_token",
					Value: base64.StdEncoding.EncodeToString(validSignature),
				})
			},
			expectUserID:   true,
			expectedUserID: testdata.TestUserID,
		},
		{
			name: "Invalid signature in header",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "invalid-signature")
			},
			expectUserID: false,
		},
		{
			name: "No signature provided",
			setupRequest: func(r *http.Request) {
				// No auth headers or cookies
			},
			expectUserID: false,
		},
		{
			name: "Header takes precedence over cookie",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", base64.StdEncoding.EncodeToString(validSignature))
				r.AddCookie(&http.Cookie{
					Name:  "auth_token",
					Value: "invalid-cookie-signature",
				})
			},
			expectUserID:   true,
			expectedUserID: testdata.TestUserID,
		},
		{
			name: "Invalid header, valid cookie - header takes precedence",
			setupRequest: func(r *http.Request) {
				r.Header.Set("Authorization", "invalid-header-signature")
				r.AddCookie(&http.Cookie{
					Name:  "auth_token",
					Value: base64.StdEncoding.EncodeToString(validSignature),
				})
			},
			expectUserID: false, // Invalid header prevents cookie fallback
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			tc.setupRequest(req)

			recorder := httptest.NewRecorder()
			var capturedContext context.Context

			// Create a test handler that captures the context
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedContext = r.Context()
				w.WriteHeader(http.StatusOK)
			})

			middleware := provider.WithHeaderAuthorization()
			wrappedHandler := middleware(testHandler)
			wrappedHandler.ServeHTTP(recorder, req)

			// Check if user ID was set in context
			userID := capturedContext.Value(ContextKeyUserID)
			if tc.expectUserID {
				if userID == nil {
					t.Error("Expected user ID in context, but got none")
				} else if userID.(model.UserID) != tc.expectedUserID {
					t.Errorf(errExpectedUserIDGotAnother, tc.expectedUserID, userID.(model.UserID))
				}
			} else {
				if userID != nil {
					t.Errorf("Expected no user ID in context, but got '%s'", userID.(model.UserID))
				}
			}

			// All requests should result in 200 OK (middleware doesn't block)
			if recorder.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", recorder.Code)
			}
		})
	}
}

func TestEd25519AuthProvider_GetUserIDFromSession(t *testing.T) {
	provider, err := NewEd25519AuthProvider(testdata.TestPublicKeyPEM, "Authorization", testdata.TestUserID)
	if err != nil {
		t.Fatalf(failedToCreateProvider, err)
	}

	testCases := []struct {
		name        string
		context     context.Context
		expectError bool
		expectedID  model.UserID
	}{
		{
			name:        "Context with user ID",
			context:     ContextWithUserID(context.Background(), testdata.TestUserID),
			expectError: false,
			expectedID:  testdata.TestUserID,
		},
		{
			name:        "Context without user ID",
			context:     context.Background(),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req = req.WithContext(tc.context)

			userID, err := provider.GetUserIDFromSession(req)

			if tc.expectError {
				if err == nil {
					t.Error(errExpectedErrorGotNone)
				}
				return
			}

			if err != nil {
				t.Errorf(errUnexpected, err)
				return
			}

			if userID != tc.expectedID {
				t.Errorf(errExpectedUserIDGotAnother, tc.expectedID, userID)
			}
		})
	}
}

func TestEd25519AuthProvider_EnforceUserAndGetID(t *testing.T) {
	provider, err := NewEd25519AuthProvider(testdata.TestPublicKeyPEM, "Authorization", testdata.TestUserID)
	if err != nil {
		t.Fatalf(failedToCreateProvider, err)
	}

	testCases := []struct {
		name           string
		context        context.Context
		expectError    bool
		expectedID     model.UserID
		expectedStatus int
		expectRedirect bool
	}{
		{
			name:           "Authorized user",
			context:        ContextWithUserID(context.Background(), testdata.TestUserID),
			expectError:    false,
			expectedID:     testdata.TestUserID,
			expectedStatus: 0, // No HTTP response written
		},
		{
			name:           "Unauthorized user",
			context:        context.Background(),
			expectError:    true,
			expectedStatus: http.StatusUnauthorized,
			expectRedirect: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req = req.WithContext(tc.context)
			recorder := httptest.NewRecorder()

			userID, err := provider.EnforceUserAndGetID(recorder, req)

			if tc.expectError {
				if err == nil {
					t.Error(errExpectedErrorGotNone)
				}
				if recorder.Code != tc.expectedStatus {
					t.Errorf("Expected status %d, got %d", tc.expectedStatus, recorder.Code)
				}
				if tc.expectRedirect {
					redirect := recorder.Header().Get("Hx-Redirect")
					if redirect != "/auth/login" {
						t.Errorf("Expected Hx-Redirect to '/auth/login', got '%s'", redirect)
					}
				}
				return
			}

			if err != nil {
				t.Errorf(errUnexpected, err)
				return
			}

			if userID != tc.expectedID {
				t.Errorf(errExpectedUserIDGotAnother, tc.expectedID, userID)
			}

			if recorder.Code != 200 && recorder.Code != 0 {
				t.Errorf("Expected no HTTP error response, got status %d", recorder.Code)
			}
		})
	}
}

func TestEd25519AuthProvider_GetChallenge(t *testing.T) {
	provider, err := NewEd25519AuthProvider(testdata.TestPublicKeyPEM, "Authorization", testdata.TestUserID)
	if err != nil {
		t.Fatalf(failedToCreateProvider, err)
	}

	challenge := provider.GetChallenge()
	if len(challenge) != 32 {
		t.Errorf("Expected challenge length 32, got %d", len(challenge))
	}
	if challenge == nil {
		t.Error("Expected challenge to be non-nil")
	}
}

func TestEd25519AuthProvider_RefreshChallenge(t *testing.T) {
	provider, err := NewEd25519AuthProvider(testdata.TestPublicKeyPEM, "Authorization", testdata.TestUserID)
	if err != nil {
		t.Fatalf(failedToCreateProvider, err)
	}

	originalChallenge := make([]byte, len(provider.challenge))
	copy(originalChallenge, provider.challenge)

	err = provider.RefreshChallenge()
	if err != nil {
		t.Errorf("Unexpected error refreshing challenge: %v", err)
	}

	newChallenge := provider.GetChallenge()
	if len(newChallenge) != 32 {
		t.Errorf("Expected new challenge length 32, got %d", len(newChallenge))
	}

	// Challenges should be different (very unlikely to be the same)
	if string(originalChallenge) == string(newChallenge) {
		t.Error("Expected new challenge to be different from original")
	}
}

func TestEd25519AuthProvider_HandleWebhookUser(t *testing.T) {
	provider, err := NewEd25519AuthProvider(testdata.TestPublicKeyPEM, "Authorization", testdata.TestUserID)
	if err != nil {
		t.Fatalf(failedToCreateProvider, err)
	}

	req := httptest.NewRequest("POST", "/webhook", nil)
	recorder := httptest.NewRecorder()

	provider.HandleWebhookUser(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

// Helper function to generate a valid signature for testing
func generateValidSignature(t *testing.T, challenge []byte) []byte {
	// Parse the test private key
	block, _ := pem.Decode([]byte(testdata.TestPrivateKeyPEM))
	if block == nil {
		t.Fatal("Failed to parse private key PEM")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	ed25519PrivateKey, ok := privateKey.(ed25519.PrivateKey)
	if !ok {
		t.Fatal("Private key is not Ed25519")
	}

	// Sign the challenge
	signature := ed25519.Sign(ed25519PrivateKey, challenge)
	return signature
}
