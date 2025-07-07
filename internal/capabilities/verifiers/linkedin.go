package verifiers

import (
	"context"
	"fmt"

	linkedinscraper "github.com/masa-finance/linkedin-scraper"
	"github.com/masa-finance/tee-worker/api/types"
)

const verificationLinkedInProfileURL = "https://www.linkedin.com/in/williamhgates/"

// LinkedInVerifier verifies the LinkedIn capability.
type LinkedInVerifier struct {
	client *linkedinscraper.Client
}

// NewLinkedInVerifier creates a new LinkedInVerifier.
func NewLinkedInVerifier(creds []types.LinkedInCredential) (*LinkedInVerifier, error) {
	if len(creds) == 0 {
		return nil, fmt.Errorf("no linkedin credentials provided for verification")
	}

	// Use the first available credential for verification
	cred := creds[0]
	authCreds := linkedinscraper.AuthCredentials{
		LiAtCookie: cred.LiAtCookie,
		CSRFToken:  cred.CSRFToken,
		JSESSIONID: cred.JSESSIONID,
	}

	cfg, err := linkedinscraper.NewConfig(authCreds)
	if err != nil {
		return nil, fmt.Errorf("verifier: failed to create linkedin config: %w", err)
	}

	client, err := linkedinscraper.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("verifier: failed to create linkedin client: %w", err)
	}

	return &LinkedInVerifier{
		client: client,
	}, nil
}

// Verify attempts to fetch a well-known public profile.
func (v *LinkedInVerifier) Verify(ctx context.Context) (bool, error) {
	if v.client == nil {
		return false, fmt.Errorf("linkedin verifier not initialized")
	}

	profile, err := v.client.GetProfile(ctx, verificationLinkedInProfileURL)
	if err != nil {
		return false, err
	}

	if profile.PublicIdentifier == "" {
		return false, fmt.Errorf("verification profile is empty")
	}

	return true, nil
}
