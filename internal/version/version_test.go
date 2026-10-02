package version

import "testing"

func TestPseudoVersion(t *testing.T) {
	m := pseudoVersion.FindStringSubmatch("v0.0.0-20261002113744-78a350731258+dirty")
	if m == nil || m[1][:7] != "78a3507" || m[2] != "+dirty" {
		t.Fatalf("%v", m)
	}
	if pseudoVersion.MatchString("v1.2.3") {
		t.Fatal("tag matched as pseudo-version")
	}
}
