package main

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Cacsjep/goxis/pkg/vapix"
)

var httpSOAPClient = &http.Client{Timeout: 30 * time.Second}

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

	resp, err := httpSOAPClient.Do(req)
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
		return respBody, errors.New(msg)
	}

	if resp.StatusCode != 200 {
		return respBody, fmt.Errorf("SOAP request returned status %d", resp.StatusCode)
	}

	return respBody, nil
}

// InstallCertToCamera uploads the lego certificate and private key to the camera
// and configures it as the HTTPS certificate. Uses a timestamped cert ID so each
// install gets a unique ID, then cleans up old lego- certs after switching HTTPS.
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

	// Generate unique cert ID with timestamp (e.g. "lego-260215143025")
	certID := legoCertIDPrefix + time.Now().Format("060102150405")

	// Step 1: Upload new certificate with unique ID
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

	// Step 2: Fetch available ciphers
	ciphers, err := fetchCiphers(username, password)
	if err != nil {
		return fmt.Errorf("failed to fetch ciphers: %w", err)
	}

	// Step 3: Set HTTPS to use the new certificate
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

	// Step 4: Clean up old lego- certs (best-effort, the new cert is already active)
	cleanupOldLegoCerts(username, password, certID)

	return nil
}

// cleanupOldLegoCerts lists all certificates on the camera and deletes any
// with IDs starting with "lego-" that aren't the current active cert.
func cleanupOldLegoCerts(username, password, currentCertID string) {
	ids, err := listCertificateIDs(username, password)
	if err != nil {
		return
	}
	for _, id := range ids {
		if strings.HasPrefix(id, legoCertIDPrefix) && id != currentCertID {
			deleteCert(username, password, id)
		}
	}
}

// listCertificateIDs retrieves all certificate IDs from the camera via ONVIF GetCertificates.
func listCertificateIDs(username, password string) ([]string, error) {
	body := `<tds:GetCertificates xmlns="http://www.onvif.org/ver10/device/wsdl"/>`
	resp, err := vapixSOAPPost(username, password, body)
	if err != nil {
		return nil, err
	}
	return extractCertIDs(string(resp)), nil
}

// extractCertIDs parses certificate IDs from a GetCertificates SOAP response.
func extractCertIDs(xml string) []string {
	var ids []string
	search := xml
	for {
		start := strings.Index(search, "<tt:CertificateID>")
		if start < 0 {
			break
		}
		start += len("<tt:CertificateID>")
		end := strings.Index(search[start:], "</tt:CertificateID>")
		if end < 0 {
			break
		}
		id := strings.TrimSpace(search[start : start+end])
		if id != "" {
			ids = append(ids, id)
		}
		search = search[start+end:]
	}
	return ids
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
