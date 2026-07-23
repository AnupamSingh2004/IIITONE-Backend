package auth

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Identity is the set of claims we need from a verified Google ID token.
type Identity struct {
	Email string
	HD    string
	Sub   string
	Name  string
}

// CodeVerifier exchanges an OAuth code for a verified identity. Implemented
// by GoogleVerifier in production and a fake in tests.
type CodeVerifier interface {
	VerifyCode(ctx context.Context, code string) (Identity, error)
}

type GoogleVerifier struct {
	oauthConfig *oauth2.Config
	provider    *oidc.Provider
	verifier    *oidc.IDTokenVerifier
}

func NewGoogleVerifier(ctx context.Context, clientID, clientSecret, redirectURL string) (*GoogleVerifier, error) {
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, err
	}
	return &GoogleVerifier{
		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

// LoginURL returns the Google OAuth consent-screen URL for the given
// anti-CSRF state value.
func (g *GoogleVerifier) LoginURL(state string) string {
	return g.oauthConfig.AuthCodeURL(state)
}

func (g *GoogleVerifier) VerifyCode(ctx context.Context, code string) (Identity, error) {
	token, err := g.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return Identity{}, err
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return Identity{}, errNoIDToken
	}
	idToken, err := g.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, err
	}
	var claims struct {
		Email string `json:"email"`
		HD    string `json:"hd"`
		Name  string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, err
	}
	return Identity{Email: claims.Email, HD: claims.HD, Sub: idToken.Subject, Name: claims.Name}, nil
}
