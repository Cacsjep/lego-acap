package main

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Cacsjep/goxis/pkg/vapix"
)

const (
	vapixServicesPath = "/vapix/services"
	legoCertIDPrefix  = "lego-"
)

func soapEnvelope(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://www.w3.org/2003/05/soap-envelope"
    xmlns:tds="http://www.onvif.org/ver10/device/wsdl"
    xmlns:tt="http://www.onvif.org/ver10/schema">
  <SOAP-ENV:Body>` + body + `</SOAP-ENV:Body>
</SOAP-ENV:Envelope>`
}

func vapixSOAPPost(username, password, body string) ([]byte, error) {
	envelope := soapEnvelope(body)
	url := vapix.InternalVapixUrlPathJoin(vapixServicesPath)

	req, err := http.NewRequest("POST", url, bytes.NewBufferString(envelope))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/soap+xml")
	req.SetBasicAuth(username, password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SOAP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	respStr := string(respBody)

	// Check for SOAP Fault (works for both 200 and non-200 responses)
	if strings.Contains(respStr, "Fault") {
		reason := extractSOAPText(respStr, "Reason")
		detail := extractSOAPText(respStr, "Detail")
		msg := "SOAP fault"
		if reason != "" {
			msg = reason
		}
		if detail != "" {
			msg += ": " + detail
		}
		return respBody, fmt.Errorf("%s", msg)
	}

	if resp.StatusCode != 200 {
		return respBody, fmt.Errorf("SOAP request returned status %d", resp.StatusCode)
	}

	return respBody, nil
}

// InstallCertToCamera uploads the lego certificate and private key to the camera
// and configures it as the HTTPS certificate.
func InstallCertToCamera(username, password, domain string) error {
	certFile := legoCertsPath + "/certificates/" + domain + ".crt"
	keyFile := legoCertsPath + "/certificates/" + domain + ".key"

	// Read and encode certificate
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}
	certB64 := base64.StdEncoding.EncodeToString(certBlock.Bytes)

	// Read and encode private key (convert to PKCS#8)
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("failed to decode private key PEM")
	}

	pkcs8Key, err := toPKCS8(keyBlock)
	if err != nil {
		return fmt.Errorf("failed to convert key to PKCS#8: %w", err)
	}
	keyB64 := base64.StdEncoding.EncodeToString(pkcs8Key)

	certID := legoCertIDPrefix + sanitizeCertID(domain)
	if len(certID) > 32 {
		certID = certID[:32]
	}

	// Step 1: Delete old lego cert if exists (ignore errors)
	deleteCert(username, password, certID)

	// Step 2: Upload certificate with private key
	uploadBody := fmt.Sprintf(`
    <tds:LoadCertificateWithPrivateKey xmlns="http://www.onvif.org/ver10/device/wsdl">
      <CertificateWithPrivateKey>
        <tt:CertificateID>%s</tt:CertificateID>
        <tt:Certificate><tt:Data>%s</tt:Data></tt:Certificate>
        <tt:PrivateKey><tt:Data>%s</tt:Data></tt:PrivateKey>
      </CertificateWithPrivateKey>
    </tds:LoadCertificateWithPrivateKey>`, certID, certB64, keyB64)

	if _, err := vapixSOAPPost(username, password, uploadBody); err != nil {
		return fmt.Errorf("failed to upload certificate: %w", err)
	}

	// Step 3: Fetch available ciphers
	ciphers, err := fetchCiphers(username, password)
	if err != nil {
		return fmt.Errorf("failed to fetch ciphers: %w", err)
	}

	// Step 4: Set as HTTPS certificate
	var cipherXML string
	for _, c := range ciphers {
		c = strings.TrimSpace(c)
		if c != "" {
			cipherXML += fmt.Sprintf("<acert:Cipher>%s</acert:Cipher>", xmlEscape(c))
		}
	}

	httpsBody := fmt.Sprintf(`
    <SetWebServerTlsConfiguration xmlns="http://www.axis.com/vapix/ws/webserver"
        xmlns:acert="http://www.axis.com/vapix/ws/cert">
      <Configuration>
        <Tls>true</Tls>
        <ConnectionPolicies><Admin>HttpAndHttps</Admin></ConnectionPolicies>
        <Ciphers>%s</Ciphers>
        <CertificateSet>
          <acert:Certificates><acert:Id>%s</acert:Id></acert:Certificates>
          <acert:CACertificates></acert:CACertificates>
          <acert:TrustedCertificates></acert:TrustedCertificates>
        </CertificateSet>
      </Configuration>
    </SetWebServerTlsConfiguration>`, cipherXML, certID)

	if _, err := vapixSOAPPost(username, password, httpsBody); err != nil {
		return fmt.Errorf("failed to set HTTPS configuration: %w", err)
	}

	return nil
}

func deleteCert(username, password, certID string) {
	body := fmt.Sprintf(`
    <tds:DeleteCertificates xmlns="http://www.onvif.org/ver10/device/wsdl">
      <CertificateID>%s</CertificateID>
    </tds:DeleteCertificates>`, certID)
	vapixSOAPPost(username, password, body)
}

func fetchCiphers(username, password string) ([]string, error) {
	url := vapix.InternalVapixUrlPathJoin("/axis-cgi/param.cgi?action=list&group=HTTPS.Ciphers")
	result := vapix.VapixGet(username, password, url)
	if !result.IsOk {
		return nil, fmt.Errorf("failed to fetch ciphers: %v", result.Error)
	}
	defer result.ResponseReader.Close()

	body, err := io.ReadAll(result.ResponseReader)
	if err != nil {
		return nil, err
	}

	// Response format: root.HTTPS.Ciphers=CIPHER1:CIPHER2:...
	line := strings.TrimSpace(string(body))
	idx := strings.Index(line, "=")
	if idx < 0 {
		return nil, fmt.Errorf("unexpected cipher response: %s", line)
	}
	cipherStr := line[idx+1:]
	return strings.Split(cipherStr, ":"), nil
}

func toPKCS8(block *pem.Block) ([]byte, error) {
	var key interface{}
	var err error

	switch block.Type {
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		// Already PKCS#8
		return block.Bytes, nil
	default:
		return nil, fmt.Errorf("unsupported key type: %s", block.Type)
	}
	if err != nil {
		return nil, err
	}
	return x509.MarshalPKCS8PrivateKey(key)
}

func sanitizeCertID(domain string) string {
	r := strings.NewReplacer(".", "-", "*", "wc")
	return r.Replace(domain)
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// extractSOAPText extracts the text content from a SOAP element like
// <SOAP-ENV:Reason><SOAP-ENV:Text xml:lang="en">Invalid ID</SOAP-ENV:Text></SOAP-ENV:Reason>
func extractSOAPText(xml, element string) string {
	// Look for <SOAP-ENV:{element}> ... <SOAP-ENV:Text ...>VALUE</SOAP-ENV:Text>
	start := strings.Index(xml, "<SOAP-ENV:"+element+">")
	if start < 0 {
		return ""
	}
	sub := xml[start:]
	// Find <SOAP-ENV:Text (with possible attributes like xml:lang="en")
	textStart := strings.Index(sub, "<SOAP-ENV:Text")
	if textStart < 0 {
		return ""
	}
	// Skip past the closing >
	gt := strings.Index(sub[textStart:], ">")
	if gt < 0 {
		return ""
	}
	contentStart := textStart + gt + 1
	// Find closing </SOAP-ENV:Text>
	contentEnd := strings.Index(sub[contentStart:], "</SOAP-ENV:Text>")
	if contentEnd < 0 {
		return ""
	}
	return strings.TrimSpace(sub[contentStart : contentStart+contentEnd])
}
