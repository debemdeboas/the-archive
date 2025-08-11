package testdata

// Test Ed25519 key pair for testing purposes only
// DO NOT USE IN PRODUCTION

const TestPrivateKeyPEM = `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIKnCM5WDRjDG+An5du7O+j32rUfB6kUmQTJYibkn+X+T
-----END PRIVATE KEY-----`

const TestPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAfnFj+XvGh8tXwcDcw8gGblS+7rnWn65V1RNajNg0CC4=
-----END PUBLIC KEY-----`

// Test challenge for consistent testing
var TestChallenge = []byte("test-challenge-for-auth-testing-purposes-do-not-use-in-production")

// Test user ID for testing
const TestUserID = "test-user-123"
