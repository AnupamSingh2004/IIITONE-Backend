package search

import "github.com/AnupamSingh2004/iiitone-backend/internal/auth"

func testIdentity() auth.Identity {
	return auth.Identity{Email: "search-test@iiitdmj.ac.in", HD: "iiitdmj.ac.in", Sub: "search-test-sub", Name: "Search Tester"}
}
