package user

import (
	"fmt"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/user"
)

// ClerkProvider is a user provider that uses Clerk to manage users.
type ClerkAuthProvider struct {
}

func GetSessionKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := clerk.SessionClaimsFromContext(ctx)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	usr, err := user.Get(ctx, claims.Subject)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Println(usr)
}
