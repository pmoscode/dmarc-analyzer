package report

import "fmt"

// PublishedPolicy ist die vom Domaininhaber veröffentlichte DMARC-Policy
// (Element "policy_published"), gegen die der berichtende Empfänger
// bewertet hat.
type PublishedPolicy struct {
	Domain          DomainName
	SubdomainPolicy Policy
	Policy          Policy
	DKIMAlignment   AlignmentMode
	SPFAlignment    AlignmentMode
	Percentage      int
	FailureOptions  string
}

// NewPublishedPolicy erzwingt Percentage in [0, 100]
// (IMPLEMENTIERUNG.md Abschnitt 6.2).
func NewPublishedPolicy(
	domain DomainName,
	subdomainPolicy, policy Policy,
	dkimAlignment, spfAlignment AlignmentMode,
	percentage int,
	failureOptions string,
) (PublishedPolicy, error) {
	if percentage < 0 || percentage > 100 {
		return PublishedPolicy{}, fmt.Errorf("percentage muss in [0, 100] liegen, war %d", percentage)
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
