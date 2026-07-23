package moderation

import "github.com/AnupamSingh2004/iiitone-backend/internal/auth"

func testIdentity(suffix string) auth.Identity {
	return auth.Identity{
		Email: suffix + "@iiitdmj.ac.in",
		HD:    "iiitdmj.ac.in",
		Sub:   "sub-" + suffix,
		Name:  "Test User " + suffix,
	}
}
