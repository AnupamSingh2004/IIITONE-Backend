package auth

import "testing"

func TestValidateCollegeIdentity(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		hd        string
		wantValid bool
	}{
		{"valid iiitdmj account", "student@iiitdmj.ac.in", "iiitdmj.ac.in", true},
		{"hd claim mismatched despite matching-looking email", "student@iiitdmj.ac.in.evil.com", "evil.com", false},
		{"missing hd claim entirely", "student@iiitdmj.ac.in", "", false},
		{"correct hd but wrong email suffix", "student@gmail.com", "iiitdmj.ac.in", false},
		{"both wrong", "student@gmail.com", "gmail.com", false},
		{"subdomain email does not satisfy the parent-domain hd claim", "student@sub.iiitdmj.ac.in", "iiitdmj.ac.in", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateCollegeIdentity(tt.email, tt.hd, "iiitdmj.ac.in")
			if got != tt.wantValid {
				t.Errorf("ValidateCollegeIdentity(%q, %q) = %v, want %v", tt.email, tt.hd, got, tt.wantValid)
			}
		})
	}
}

func TestValidateCollegeIdentity_EmptyAllowedDomainFailsClosed(t *testing.T) {
	// A misconfigured (empty) allowed domain must deny everyone, not allow-all.
	if ValidateCollegeIdentity("student@iiitdmj.ac.in", "", "") {
		t.Error("ValidateCollegeIdentity with empty allowedDomain must return false, got true")
	}
}
