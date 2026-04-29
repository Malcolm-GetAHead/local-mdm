package windows

import (
	"encoding/xml"
	"fmt"

	"github.com/google/uuid"
)

// MS-MDE2 discovery namespace and action URIs
const (
	DiscoveryNS        = "http://schemas.microsoft.com/windows/management/2012/01/enrollment"
	DiscoveryAction    = "http://schemas.microsoft.com/windows/management/2012/01/enrollment/IDiscoveryService/Discover"
	DiscoverRespAction = "http://schemas.microsoft.com/windows/management/2012/01/enrollment/IDiscoveryService/DiscoverResponse"

	soapNS       = "http://www.w3.org/2003/05/soap-envelope"
	addressingNS = "http://www.w3.org/2005/08/addressing"
	activityNS   = "http://schemas.microsoft.com/2004/09/ServiceModel/Diagnostics"
	xsiNS        = "http://www.w3.org/2001/XMLSchema-instance"
	xsdNS        = "http://www.w3.org/2001/XMLSchema"
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

// --- Response structs for xml.Marshal ---

type discoverRespEnvelope struct {
	XMLName xml.Name          `xml:"s:Envelope"`
	XMLNS_S string            `xml:"xmlns:s,attr"`
	XMLNS_A string            `xml:"xmlns:a,attr"`
	Header  discoverRespHdr   `xml:"s:Header"`
	Body    discoverRespBody  `xml:"s:Body"`
}

type discoverRespHdr struct {
	Action     discoverRespAction `xml:"a:Action"`
	ActivityId discoverActivityId `xml:"ActivityId"`
	RelatesTo  string             `xml:"a:RelatesTo"`
}

type discoverRespAction struct {
	MustUnderstand string `xml:"s:mustUnderstand,attr"`
	Value          string `xml:",chardata"`
}

type discoverActivityId struct {
	XMLNS string `xml:"xmlns,attr"`
	Value string `xml:",chardata"`
}

type discoverRespBody struct {
	XMLNS_XSI        string            `xml:"xmlns:xsi,attr"`
	XMLNS_XSD        string            `xml:"xmlns:xsd,attr"`
	DiscoverResponse discoverRespInner `xml:"DiscoverResponse"`
}

type discoverRespInner struct {
	XMLNS          string         `xml:"xmlns,attr"`
	DiscoverResult discoverResult `xml:"DiscoverResult"`
}

type discoverResult struct {
	AuthPolicy                 string `xml:"AuthPolicy"`
	EnrollmentVersion          string `xml:"EnrollmentVersion"`
	EnrollmentPolicyServiceUrl string `xml:"EnrollmentPolicyServiceUrl"`
	EnrollmentServiceUrl       string `xml:"EnrollmentServiceUrl"`
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
	env := discoverRespEnvelope{
		XMLNS_S: soapNS,
		XMLNS_A: addressingNS,
		Header: discoverRespHdr{
			Action: discoverRespAction{
				MustUnderstand: "1",
				Value:          DiscoverRespAction,
			},
			ActivityId: discoverActivityId{
				XMLNS: activityNS,
				Value: uuid.New().String(),
			},
			RelatesTo: relatesToMessageID,
		},
		Body: discoverRespBody{
			XMLNS_XSI: xsiNS,
			XMLNS_XSD: xsdNS,
			DiscoverResponse: discoverRespInner{
				XMLNS: DiscoveryNS,
				DiscoverResult: discoverResult{
					AuthPolicy:                 "OnPremise",
					EnrollmentVersion:          "4.0",
					EnrollmentPolicyServiceUrl: policyURL,
					EnrollmentServiceUrl:       enrollmentURL,
				},
			},
		},
	}

	data, err := xml.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal discovery response: %w", err)
	}
	return append([]byte(xml.Header), data...), nil
}
