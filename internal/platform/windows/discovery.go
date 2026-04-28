package windows

import (
	"encoding/xml"
	"fmt"
)

// MS-MDE2 discovery namespace and action URIs
const (
	DiscoveryNS       = "http://schemas.microsoft.com/windows/management/2012/01/enrollment/"
	DiscoveryAction   = "http://schemas.microsoft.com/windows/management/2012/01/enrollment/IDiscoveryService/Discover"
	DiscoverRespAction = "http://schemas.microsoft.com/windows/management/2012/01/enrollment/IDiscoveryService/DiscoverResponse"
)

// DiscoverRequest represents the inner Discover element within the SOAP body.
type DiscoverRequest struct {
	XMLName xml.Name       `xml:"Discover"`
	Request DiscoverParams `xml:"request"`
}

// DiscoverParams holds the discovery request parameters.
type DiscoverParams struct {
	EmailAddress       string `xml:"EmailAddress"`
	RequestVersion     string `xml:"RequestVersion"`
	DeviceType         string `xml:"DeviceType"`
	ApplicationVersion string `xml:"ApplicationVersion"`
	OSEdition          string `xml:"OSEdition"`
}

// discoverSOAPEnvelope is used only for parsing the incoming SOAP-wrapped discovery request.
type discoverSOAPEnvelope struct {
	XMLName xml.Name         `xml:"Envelope"`
	Header  discoverHeader   `xml:"Header"`
	Body    discoverSOAPBody `xml:"Body"`
}

type discoverHeader struct {
	Action    string `xml:"Action"`
	MessageID string `xml:"MessageID"`
	To        string `xml:"To"`
}

type discoverSOAPBody struct {
	Discover DiscoverRequest `xml:"Discover"`
}

// ParseDiscoverRequest parses a SOAP 1.2 envelope containing a Discover request.
// Windows sends discovery inside <s:Envelope> with WS-Addressing headers.
func ParseDiscoverRequest(data []byte) (*DiscoverRequest, string, error) {
	var env discoverSOAPEnvelope
	if err := xml.Unmarshal(data, &env); err != nil {
		// Try bare XML as fallback (for tests)
		var req DiscoverRequest
		if err2 := xml.Unmarshal(data, &req); err2 != nil {
			return nil, "", fmt.Errorf("failed to parse discovery request: %w", err)
		}
		return &req, "", nil
	}
	return &env.Body.Discover, env.Header.MessageID, nil
}

// GenerateDiscoverResponse generates a SOAP 1.2 envelope discovery response
// with proper WS-Addressing headers per MS-MDE2 spec.
func GenerateDiscoverResponse(enrollmentURL, policyURL, relatesToMessageID string) ([]byte, error) {
	resp := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
   xmlns:a="http://www.w3.org/2005/08/addressing">
  <s:Header>
    <a:Action s:mustUnderstand="1">%s</a:Action>
    <a:RelatesTo>%s</a:RelatesTo>
  </s:Header>
  <s:Body xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
     xmlns:xsd="http://www.w3.org/2001/XMLSchema">
    <DiscoverResponse xmlns="%s">
      <DiscoverResult>
        <AuthPolicy>OnPremise</AuthPolicy>
        <EnrollmentVersion>5.0</EnrollmentVersion>
        <AuthenticationServiceUrl>%s</AuthenticationServiceUrl>
        <EnrollmentPolicyServiceUrl>%s</EnrollmentPolicyServiceUrl>
        <EnrollmentServiceUrl>%s</EnrollmentServiceUrl>
      </DiscoverResult>
    </DiscoverResponse>
  </s:Body>
</s:Envelope>`, DiscoverRespAction, relatesToMessageID, DiscoveryNS, enrollmentURL, policyURL, enrollmentURL)

	return []byte(resp), nil
}
