package report

import "fmt"

// PublishedPolicy is the DMARC policy published by the domain owner
// (element "policy_published") that the reporting recipient evaluated
// against.
type PublishedPolicy struct {
	Domain          DomainName
	SubdomainPolicy Policy
	Policy          Policy
	DKIMAlignment   AlignmentMode
	SPFAlignment    AlignmentMode
	Percentage      int
	FailureOptions  string
}

// NewPublishedPolicy enforces Percentage in [0, 100]
// (IMPLEMENTIERUNG.md section 6.2).
func NewPublishedPolicy(
	domain DomainName,
	subdomainPolicy, policy Policy,
	dkimAlignment, spfAlignment AlignmentMode,
	percentage int,
	failureOptions string,
) (PublishedPolicy, error) {
	if percentage < 0 || percentage > 100 {
		return PublishedPolicy{}, fmt.Errorf("percentage must be in [0, 100], was %d", percentage)
	}

	return PublishedPolicy{
		Domain:          domain,
		SubdomainPolicy: subdomainPolicy,
		Policy:          policy,
		DKIMAlignment:   dkimAlignment,
		SPFAlignment:    spfAlignment,
		Percentage:      percentage,
		FailureOptions:  failureOptions,
	}, nil
}
