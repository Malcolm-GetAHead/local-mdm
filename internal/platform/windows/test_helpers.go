package windows

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// GenerateTestCSR creates a base64-encoded DER CSR for testing.
func GenerateTestCSR(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := &x509.CertificateRequest{Subject: pkix.Name{CommonName: "TestWindowsDevice"}}
	der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(der)
}

// BuildTestEnrollmentSOAP wraps a base64 CSR in a valid MS-MDE2 WSTEP SOAP envelope.
func BuildTestEnrollmentSOAP(t *testing.T, csrB64 string) string {
	t.Helper()
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
            xmlns:a="http://www.w3.org/2005/08/addressing"
            xmlns:wsse="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd"
            xmlns:wst="http://docs.oasis-open.org/ws-sx/ws-trust/200512"
            xmlns:ac="http://schemas.xmlsoap.org/ws/2006/12/authorization">
  <s:Header>
    <a:Action s:mustUnderstand="1">http://schemas.microsoft.com/windows/pki/2009/01/enrollment/RST/wstep</a:Action>
    <a:MessageID>urn:uuid:test-enroll-001</a:MessageID>
    <a:To s:mustUnderstand="1">https://mdm.example.com/EnrollmentServer/Enrollment.svc</a:To>
    <wsse:Security>
      <wsse:UsernameToken>
        <wsse:Username>admin@localmdm.local</wsse:Username>
        <wsse:Password>dummy</wsse:Password>
      </wsse:UsernameToken>
    </wsse:Security>
  </s:Header>
  <s:Body>
    <wst:RequestSecurityToken>
      <wst:TokenType>http://schemas.microsoft.com/5.0.0.0/ConfigurationManager/Enrollment/DeviceEnrollmentToken</wst:TokenType>
      <wst:RequestType>http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue</wst:RequestType>
      <wsse:BinarySecurityToken ValueType="http://schemas.microsoft.com/windows/pki/2009/01/enrollment#PKCS10"
                                EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd#base64binary">%s</wsse:BinarySecurityToken>
      <ac:AdditionalContext>
        <ac:ContextItem Name="DeviceID"><ac:Value>test-hw-device-001</ac:Value></ac:ContextItem>
        <ac:ContextItem Name="DeviceName"><ac:Value>DESKTOP-TEST</ac:Value></ac:ContextItem>
        <ac:ContextItem Name="OSVersion"><ac:Value>10.0.26200.0</ac:Value></ac:ContextItem>
      </ac:AdditionalContext>
    </wst:RequestSecurityToken>
  </s:Body>
</s:Envelope>`, csrB64)
}
