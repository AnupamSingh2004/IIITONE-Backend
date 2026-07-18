package auth

import "strings"

// ValidateCollegeIdentity enforces the spec's hard trust boundary: BOTH the
// email suffix and the token's verified `hd` (hosted domain) claim must match
// the college's Workspace domain. The hd claim is what actually proves Google
// Workspace ownership — the email suffix alone can be spoofed in string form.
func ValidateCollegeIdentity(email, hd, allowedDomain string) bool {
	if hd != allowedDomain {
		return false
	}
	return strings.HasSuffix(email, "@"+allowedDomain)
}
