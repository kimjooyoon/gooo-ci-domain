package cibudgetdomain

import _ "embed"

//go:embed policy.gooo
var policySource []byte

func SourcePolicyDigest() string { return hashBytes(policySource) }
