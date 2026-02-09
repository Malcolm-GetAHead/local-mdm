package windows

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strings"
)

// SOAPEnvelope represents a SOAP envelope
type SOAPEnvelope struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2003/05/soap-envelope Envelope"`
	Header  SOAPHeader  `xml:"Header"`
	Body    SOAPBody    `xml:"Body"`
}

// SOAPHeader represents SOAP header
type SOAPHeader struct {
	Action    string         `xml:"http://www.w3.org/2005/08/addressing Action"`
	MessageID string         `xml:"http://www.w3.org/2005/08/addressing MessageID"`
	ReplyTo   *ReplyTo       `xml:"http://www.w3.org/2005/08/addressing ReplyTo,omitempty"`
	To        string         `xml:"http://www.w3.org/2005/08/addressing To"`
	Security  *SecurityHeader `xml:"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd Security,omitempty"`
}

// ReplyTo represents WS-Addressing ReplyTo
type ReplyTo struct {
	Address string `xml:"http://www.w3.org/2005/08/addressing Address"`
}

// SecurityHeader represents WS-Security header
type SecurityHeader struct {
	BinarySecurityToken *BinarySecurityToken `xml:"BinarySecurityToken,omitempty"`
}

// BinarySecurityToken represents a binary security token
type BinarySecurityToken struct {
	ValueType    string `xml:"ValueType,attr"`
	EncodingType string `xml:"EncodingType,attr"`
	Value        string `xml:",chardata"`
}

// SOAPBody represents SOAP body
type SOAPBody struct {
	RequestSecurityToken         *RequestSecurityToken         `xml:"RequestSecurityToken,omitempty"`
	RequestSecurityTokenResponse *RequestSecurityTokenResponse `xml:"RequestSecurityTokenResponse,omitempty"`
	Fault                        *SOAPFault                    `xml:"Fault,omitempty"`
}

// SOAPFault represents a SOAP fault
type SOAPFault struct {
	Code   string `xml:"Code>Value"`
	Reason string `xml:"Reason>Text"`
}

// RequestSecurityToken represents WS-Trust RST
type RequestSecurityToken struct {
	XMLName      xml.Name `xml:"http://docs.oasis-open.org/ws-sx/ws-trust/200512 RequestSecurityToken"`
	TokenType    string   `xml:"TokenType"`
	RequestType  string   `xml:"RequestType"`
	BinarySecurityToken *BinarySecurityToken `xml:"BinarySecurityToken,omitempty"`
	AdditionalContext   *AdditionalContext   `xml:"AdditionalContext,omitempty"`
}

// AdditionalContext represents additional enrollment context
type AdditionalContext struct {
	ContextItems []ContextItem `xml:"ContextItem"`
}

// ContextItem represents a context item
type ContextItem struct {
	Name  string `xml:"Name,attr"`
	Value string `xml:"Value"`
}

// RequestSecurityTokenResponse represents WS-Trust RSTR
type RequestSecurityTokenResponse struct {
	XMLName           xml.Name `xml:"http://docs.oasis-open.org/ws-sx/ws-trust/200512 RequestSecurityTokenResponse"`
	TokenType         string   `xml:"TokenType"`
	DispositionMessage string  `xml:"DispositionMessage,omitempty"`
	RequestedSecurityToken *RequestedSecurityToken `xml:"RequestedSecurityToken"`
	RequestID         string   `xml:"RequestID,omitempty"`
}

// RequestedSecurityToken represents the issued token
type RequestedSecurityToken struct {
	BinarySecurityToken *BinarySecurityToken `xml:"BinarySecurityToken"`
}

// ParseEnrollmentRequest parses a WSTEP enrollment request
func ParseEnrollmentRequest(data []byte) (*SOAPEnvelope, error) {
	var env SOAPEnvelope
	if err := xml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}
	return &env, nil
}

// ExtractCSR extracts the CSR from enrollment request
func ExtractCSR(env *SOAPEnvelope) ([]byte, error) {
	if env.Body.RequestSecurityToken == nil {
		return nil, fmt.Errorf("no RequestSecurityToken in body")
	}

	rst := env.Body.RequestSecurityToken
	if rst.BinarySecurityToken == nil {
		return nil, fmt.Errorf("no BinarySecurityToken in request")
	}

	// Decode base64 CSR
	csrData, err := base64.StdEncoding.DecodeString(rst.BinarySecurityToken.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode CSR: %w", err)
	}

	return csrData, nil
}

// GenerateEnrollmentResponse generates a WSTEP enrollment response
func GenerateEnrollmentResponse(cert *x509.Certificate, provisioningXML string) ([]byte, error) {
	// Encode certificate as base64
	certB64 := base64.StdEncoding.EncodeToString(cert.Raw)

	env := SOAPEnvelope{
		Header: SOAPHeader{
			Action:    "http://schemas.microsoft.com/windows/pki/2009/01/enrollment/RSTRC/wstep",
			MessageID: "urn:uuid:" + generateUUID(),
			To:        "http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous",
		},
		Body: SOAPBody{
			RequestSecurityTokenResponse: &RequestSecurityTokenResponse{
				TokenType: "http://schemas.microsoft.com/5.0.0.0/ConfigurationManager/Enrollment/DeviceEnrollmentToken",
				DispositionMessage: "Device enrolled successfully",
				RequestedSecurityToken: &RequestedSecurityToken{
					BinarySecurityToken: &BinarySecurityToken{
						ValueType:    "http://schemas.microsoft.com/5.0.0.0/ConfigurationManager/Enrollment/DeviceEnrollmentToken",
						EncodingType: "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd#base64binary",
						Value:        certB64,
					},
				},
			},
		},
	}

	data, err := xml.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return append([]byte(xml.Header), data...), nil
}

// GenerateProvisioningXML generates Windows provisioning XML
func GenerateProvisioningXML(serverURL, certThumbprint string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<wap-provisioningdoc version="1.1">
  <characteristic type="CertificateStore">
    <characteristic type="Root">
      <characteristic type="System">
        <characteristic type="%s">
          <parm name="EncodedCertificate" value=""/>
        </characteristic>
      </characteristic>
    </characteristic>
  </characteristic>
  <characteristic type="APPLICATION">
    <parm name="APPID" value="w7"/>
    <parm name="PROVIDER-ID" value="LocalMDM"/>
    <parm name="NAME" value="LocalMDM"/>
    <parm name="ADDR" value="%s"/>
    <parm name="CONNRETRYFREQ" value="6"/>
    <parm name="INITIALBACKOFFTIME" value="30000"/>
    <parm name="MAXBACKOFFTIME" value="120000"/>
    <parm name="BACKCOMPATRETRYDISABLED"/>
    <parm name="DEFAULTENCODING" value="application/vnd.syncml.dm+xml"/>
    <parm name="SSLCLIENTCERTSEARCHCRITERIA" value="Subject=CN=MDMDeviceCert&amp;Stores=MY%%5CUser"/>
    <characteristic type="APPAUTH">
      <parm name="AAUTHLEVEL" value="CLIENT"/>
      <parm name="AAUTHTYPE" value="DIGEST"/>
      <parm name="AAUTHSECRET" value="dummy"/>
      <parm name="AAUTHDATA" value="nonce"/>
    </characteristic>
    <characteristic type="APPAUTH">
      <parm name="AAUTHLEVEL" value="APPSRV"/>
      <parm name="AAUTHTYPE" value="DIGEST"/>
      <parm name="AAUTHNAME" value="dummy"/>
      <parm name="AAUTHSECRET" value="dummy"/>
      <parm name="AAUTHDATA" value="nonce"/>
    </characteristic>
  </characteristic>
  <characteristic type="DMClient">
    <characteristic type="Provider">
      <characteristic type="LocalMDM">
        <parm name="EntDeviceName" value="LocalMDM"/>
        <characteristic type="Poll">
          <parm name="NumberOfFirstRetries" value="8"/>
          <parm name="IntervalForFirstSetOfRetries" value="15"/>
          <parm name="NumberOfSecondRetries" value="5"/>
          <parm name="IntervalForSecondSetOfRetries" value="3"/>
          <parm name="NumberOfRemainingScheduledRetries" value="0"/>
          <parm name="IntervalForRemainingScheduledRetries" value="1560"/>
          <parm name="PollOnLogin" value="true"/>
        </characteristic>
      </characteristic>
    </characteristic>
  </characteristic>
</wap-provisioningdoc>`, certThumbprint, serverURL)
}

func generateUUID() string {
	// Simple UUID generation for message IDs
	return strings.ReplaceAll(fmt.Sprintf("%x-%x-%x-%x-%x",
		randomBytes(4), randomBytes(2), randomBytes(2), randomBytes(2), randomBytes(6)), " ", "")
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	// In production, use crypto/rand
	for i := range b {
		b[i] = byte(i)
	}
	return b
}
