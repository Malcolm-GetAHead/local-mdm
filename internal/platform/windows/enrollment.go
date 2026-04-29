package windows

import (
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"

	"github.com/google/uuid"
)

// SOAP/WS-Trust types for enrollment

// SOAPEnvelope represents a SOAP 1.2 envelope for enrollment messages.
type SOAPEnvelope struct {
	XMLName xml.Name   `xml:"http://www.w3.org/2003/05/soap-envelope Envelope"`
	Header  SOAPHeader `xml:"Header"`
	Body    SOAPBody   `xml:"Body"`
}

// SOAPHeader represents SOAP header with WS-Addressing.
type SOAPHeader struct {
	Action    string   `xml:"http://www.w3.org/2005/08/addressing Action"`
	MessageID string   `xml:"http://www.w3.org/2005/08/addressing MessageID"`
	ReplyTo   *ReplyTo `xml:"http://www.w3.org/2005/08/addressing ReplyTo,omitempty"`
	To        string   `xml:"http://www.w3.org/2005/08/addressing To"`
	Security  *SecurityHeader `xml:"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd Security,omitempty"`
}

// ReplyTo represents WS-Addressing ReplyTo.
type ReplyTo struct {
	Address string `xml:"http://www.w3.org/2005/08/addressing Address"`
}

// SecurityHeader represents WS-Security header.
type SecurityHeader struct {
	UsernameToken *UsernameToken       `xml:"UsernameToken,omitempty"`
	BinarySecurityToken *BinarySecurityToken `xml:"BinarySecurityToken,omitempty"`
}

// UsernameToken for on-premise auth.
type UsernameToken struct {
	Username string `xml:"Username"`
	Password string `xml:"Password"`
}

// BinarySecurityToken represents a binary security token.
type BinarySecurityToken struct {
	ValueType    string `xml:"ValueType,attr"`
	EncodingType string `xml:"EncodingType,attr"`
	Value        string `xml:",chardata"`
}

// SOAPBody represents SOAP body.
type SOAPBody struct {
	RequestSecurityToken *RequestSecurityToken `xml:"RequestSecurityToken,omitempty"`
	Fault                *SOAPFault            `xml:"Fault,omitempty"`
}

// SOAPFault represents a SOAP fault.
type SOAPFault struct {
	Code   string `xml:"Code>Value"`
	Reason string `xml:"Reason>Text"`
}

// RequestSecurityToken represents WS-Trust RST.
type RequestSecurityToken struct {
	XMLName             xml.Name             `xml:"http://docs.oasis-open.org/ws-sx/ws-trust/200512 RequestSecurityToken"`
	TokenType           string               `xml:"TokenType"`
	RequestType         string               `xml:"RequestType"`
	BinarySecurityToken *BinarySecurityToken `xml:"http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd BinarySecurityToken,omitempty"`
	AdditionalContext   *AdditionalContext   `xml:"http://schemas.xmlsoap.org/ws/2006/12/authorization AdditionalContext,omitempty"`
}

// AdditionalContext represents additional enrollment context.
type AdditionalContext struct {
	ContextItems []ContextItem `xml:"ContextItem"`
}

// ContextItem represents a context item.
type ContextItem struct {
	Name  string `xml:"Name,attr"`
	Value string `xml:"Value"`
}

// GetContextValue returns the value for a named context item.
func (ac *AdditionalContext) GetContextValue(name string) string {
	if ac == nil {
		return ""
	}
	for _, item := range ac.ContextItems {
		if item.Name == name {
			return item.Value
		}
	}
	return ""
}

// ParseEnrollmentRequest parses a WSTEP enrollment request SOAP envelope.
func ParseEnrollmentRequest(data []byte) (*SOAPEnvelope, error) {
	var env SOAPEnvelope
	if err := xml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}
	return &env, nil
}

// ExtractCSR extracts the DER-encoded CSR from the enrollment request.
func ExtractCSR(env *SOAPEnvelope) ([]byte, error) {
	if env.Body.RequestSecurityToken == nil {
		return nil, fmt.Errorf("no RequestSecurityToken in body")
	}
	rst := env.Body.RequestSecurityToken
	if rst.BinarySecurityToken == nil {
		return nil, fmt.Errorf("no BinarySecurityToken in request")
	}
	csrData, err := base64.StdEncoding.DecodeString(rst.BinarySecurityToken.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode CSR: %w", err)
	}
	return csrData, nil
}

// ExtractAdditionalContext returns the AdditionalContext from the enrollment request.
func ExtractAdditionalContext(env *SOAPEnvelope) *AdditionalContext {
	if env.Body.RequestSecurityToken == nil {
		return nil
	}
	return env.Body.RequestSecurityToken.AdditionalContext
}

// GenerateEnrollmentResponse generates a WSTEP enrollment response per MS-MDE2 spec.
// The BinarySecurityToken contains base64-encoded WAP provisioning XML (not the raw cert).
// Uses RequestSecurityTokenResponseCollection wrapper.
func GenerateEnrollmentResponse(provisioningXML string, relatesToMessageID string) ([]byte, error) {
	provB64 := base64.StdEncoding.EncodeToString([]byte(provisioningXML))

	resp := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
   xmlns:a="http://www.w3.org/2005/08/addressing"
   xmlns:u="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">
  <s:Header>
    <a:Action s:mustUnderstand="1">http://schemas.microsoft.com/windows/pki/2009/01/enrollment/RSTRC/wstep</a:Action>
    <a:RelatesTo>%s</a:RelatesTo>
  </s:Header>
  <s:Body>
    <RequestSecurityTokenResponseCollection xmlns="http://docs.oasis-open.org/ws-sx/ws-trust/200512">
      <RequestSecurityTokenResponse>
        <TokenType>http://schemas.microsoft.com/5.0.0.0/ConfigurationManager/Enrollment/DeviceEnrollmentToken</TokenType>
        <DispositionMessage xmlns="http://schemas.microsoft.com/windows/pki/2009/01/enrollment"></DispositionMessage>
        <RequestedSecurityToken>
          <BinarySecurityToken ValueType="http://schemas.microsoft.com/5.0.0.0/ConfigurationManager/Enrollment/DeviceEnrollmentProvisionDoc"
             EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd#base64binary"
             xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">%s</BinarySecurityToken>
        </RequestedSecurityToken>
        <RequestID xmlns="http://schemas.microsoft.com/windows/pki/2009/01/enrollment">0</RequestID>
      </RequestSecurityTokenResponse>
    </RequestSecurityTokenResponseCollection>
  </s:Body>
</s:Envelope>`, relatesToMessageID, provB64)

	return []byte(resp), nil
}

// GenerateProvisioningXML generates the WAP provisioning document per MS-MDE2 spec.
// Includes: CA cert in Root store, device cert in My/User store, WSTEP/Renew, DMClient config.
func GenerateProvisioningXML(managementURL string, caCert *x509.Certificate, deviceCert *x509.Certificate) string {
	// SHA-1 thumbprints for certificate store references (Windows uses SHA-1 for cert store keys)
	caThumbprint := fmt.Sprintf("%X", sha1.Sum(caCert.Raw))
	deviceThumbprint := fmt.Sprintf("%X", sha1.Sum(deviceCert.Raw))

	caB64 := base64.StdEncoding.EncodeToString(caCert.Raw)
	deviceB64 := base64.StdEncoding.EncodeToString(deviceCert.Raw)

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<wap-provisioningdoc version="1.1">
  <characteristic type="CertificateStore">
    <characteristic type="Root">
      <characteristic type="System">
        <characteristic type="%s">
          <parm name="EncodedCertificate" value="%s"/>
        </characteristic>
      </characteristic>
    </characteristic>
  </characteristic>
  <characteristic type="CertificateStore">
    <characteristic type="My">
      <characteristic type="User">
        <characteristic type="%s">
          <parm name="EncodedCertificate" value="%s"/>
        </characteristic>
        <characteristic type="PrivateKeyContainer"/>
      </characteristic>
      <characteristic type="WSTEP">
        <characteristic type="Renew">
          <parm name="ROBOSupport" value="true" datatype="boolean"/>
          <parm name="RenewPeriod" value="60" datatype="integer"/>
          <parm name="RetryInterval" value="4" datatype="integer"/>
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
    <parm name="SSLCLIENTCERTSEARCHCRITERIA" value="Subject=CN%%3dMDMDeviceCert&amp;Stores=MY%%5CUser"/>
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
        <parm name="EntDeviceName" value="LocalMDM" datatype="string"/>
        <characteristic type="Poll">
          <parm name="NumberOfFirstRetries" value="8" datatype="integer"/>
          <parm name="IntervalForFirstSetOfRetries" value="15" datatype="integer"/>
          <parm name="NumberOfSecondRetries" value="5" datatype="integer"/>
          <parm name="IntervalForSecondSetOfRetries" value="3" datatype="integer"/>
          <parm name="NumberOfRemainingScheduledRetries" value="0" datatype="integer"/>
          <parm name="IntervalForRemainingScheduledRetries" value="1560" datatype="integer"/>
          <parm name="PollOnLogin" value="true" datatype="boolean"/>
        </characteristic>
      </characteristic>
    </characteristic>
  </characteristic>
</wap-provisioningdoc>`, caThumbprint, caB64, deviceThumbprint, deviceB64, managementURL)
}

func generateUUID() string {
	return uuid.New().String()
}
