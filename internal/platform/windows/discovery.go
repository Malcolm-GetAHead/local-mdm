package windows

import (
	"encoding/xml"
	"fmt"
)

// DiscoverRequest represents MS-MDE2 discovery request
type DiscoverRequest struct {
	XMLName xml.Name `xml:"Discover"`
	Request struct {
		EmailAddress    string `xml:"EmailAddress"`
		RequestVersion  string `xml:"RequestVersion"`
		DeviceType      string `xml:"DeviceType"`
		ApplicationVersion string `xml:"ApplicationVersion"`
		OSEdition       string `xml:"OSEdition"`
	} `xml:"request"`
}

// DiscoverResponse represents MS-MDE2 discovery response
type DiscoverResponse struct {
	XMLName xml.Name `xml:"DiscoverResponse"`
	DiscoverResult struct {
		AuthPolicy              string `xml:"AuthPolicy"`
		EnrollmentVersion       string `xml:"EnrollmentVersion"`
		EnrollmentPolicyServiceUrl string `xml:"EnrollmentPolicyServiceUrl"`
		EnrollmentServiceUrl    string `xml:"EnrollmentServiceUrl"`
		AuthenticationServiceUrl string `xml:"AuthenticationServiceUrl,omitempty"`
	} `xml:"DiscoverResult"`
}

// ParseDiscoverRequest parses a discovery request
func ParseDiscoverRequest(data []byte) (*DiscoverRequest, error) {
	var req DiscoverRequest
	if err := xml.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to parse discovery request: %w", err)
	}
	return &req, nil
}

// GenerateDiscoverResponse generates a discovery response
func GenerateDiscoverResponse(enrollmentURL, policyURL string) ([]byte, error) {
	resp := DiscoverResponse{}
	resp.DiscoverResult.AuthPolicy = "OnPremise"
	resp.DiscoverResult.EnrollmentVersion = "5.0"
	resp.DiscoverResult.EnrollmentPolicyServiceUrl = policyURL
	resp.DiscoverResult.EnrollmentServiceUrl = enrollmentURL

	data, err := xml.MarshalIndent(resp, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery response: %w", err)
	}

	return append([]byte(xml.Header), data...), nil
}
