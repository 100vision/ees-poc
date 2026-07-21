//go:build windows

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"

	"ees-demo/common/log"
)

// Verification result details.
type verifyResult struct {
	SHA256    string // hex-encoded SHA256 hash
	Publisher string // signing certificate subject (empty if unsigned)
	SHA256OK  bool   // hash matches whitelist
	PublisherOK bool // publisher matches whitelist
}

// verifyFile computes the SHA256 hash and extracts the Authenticode publisher
// for the given file path. Returns structured results without making policy
// decisions (that's the whitelist's job).
//
// Logs each step if logger is non-nil.
func verifyFile(path string, logger *log.Logger) (*verifyResult, error) {
	if logger != nil {
		logger.Info("Verify Start: %s", path)
	}

	// Step 1: SHA256
	sha, err := sha256File(path)
	if err != nil {
		return nil, fmt.Errorf("SHA256: %w", err)
	}
	if logger != nil {
		logger.Info("SHA256: %s", sha)
	}

	// Step 2: Publisher (Authenticode)
	publisher, err := getPublisher(path)
	if err != nil {
		// Non-fatal — unsigned files are still allowed if explicitly whitelisted
		if logger != nil {
			logger.Warn("Publisher check: %v (file may be unsigned)", err)
		}
		publisher = ""
	}
	if logger != nil {
		if publisher != "" {
			logger.Info("Publisher: %s", publisher)
		} else {
			logger.Info("Publisher: (unsigned)")
		}
	}

	return &verifyResult{
		SHA256:    sha,
		Publisher: publisher,
	}, nil
}

// sha256File computes the SHA-256 hash of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := copyBuffer(h, f); err != nil {
		return "", fmt.Errorf("hash: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyBuffer is like io.Copy but written out to avoid importing io.
func copyBuffer(dst hashWriter, src *os.File) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			dst.Write(buf[:n])
			written += int64(n)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return written, nil
			}
			return written, err
		}
	}
}

type hashWriter interface {
	Write(p []byte) (int, error)
}

// containsCAMarker checks if a certificate subject name indicates a
// certificate authority (CA) rather than an end-entity (publisher) certificate.
// CA certificates typically contain "PCA", "Root", or "CA" in their names.
func containsCAMarker(name string) bool {
	for _, marker := range []string{"PCA", "Root", " CA ", " CA-", "CA ", ".CA"} {
		if containsFold(name, marker) {
			return true
		}
	}
	return false
}

// containsFold reports whether substr is within s, case-insensitively.
func containsFold(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

// equalFold is a simple ASCII case-insensitive comparison.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i]|0x20 != b[i]|0x20 {
			return false
		}
	}
	return true
}

// getPublisher extracts the signing certificate's subject name (publisher)
// from a PE file's Authenticode signature.
//
// Strategy: extract the publisher name from the embedded PKCS7 signature
// FIRST, then verify the signature chain with WinVerifyTrust. This way
// we can still extract the publisher name even if the signing certificate
// has expired (which breaks WinVerifyTrust but the name is still readable).
//
// Uses Windows APIs:
//  1. CryptQueryObject — decode the embedded PKCS7, get certificate store
//  2. CertFindCertificateInStore — find the signing certificate
//  3. CertGetNameString — extract the subject (publisher) name
//  4. WinVerifyTrust — verify the signature is currently valid
func getPublisher(path string) (string, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", fmt.Errorf("UTF16: %w", err)
	}

	// Step 1: Use CryptQueryObject to decode the embedded PKCS7 signature
	// and get a certificate store containing the signer certificates.
	var certStore windows.Handle
	var msg windows.Handle
	var context unsafe.Pointer

	err = windows.CryptQueryObject(
		windows.CERT_QUERY_OBJECT_FILE,
		unsafe.Pointer(pathPtr),
		windows.CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED,
		windows.CERT_QUERY_FORMAT_FLAG_ALL,
		0,
		nil,
		nil,
		nil,
		&certStore,
		&msg,
		&context,
	)
	if err != nil {
		return "", fmt.Errorf("CryptQueryObject: %w", err)
	}

	if certStore != 0 {
		defer windows.CertCloseStore(certStore, 0)
	}

	// Step 2: Find the signing certificate (leaf/end-entity) in the store.
	// The store may contain multiple certificates: root CA, intermediate CA(s),
	// and the actual signing certificate. The signing certificate is the one
	// whose subject name does NOT contain "PCA", "Root", or "CA" — those are
	// certificate authority names, not publisher names.
	const certEncoding = windows.X509_ASN_ENCODING | windows.PKCS_7_ASN_ENCODING

	var publisher string
	var prev *windows.CertContext

	for {
		cert, err := windows.CertFindCertificateInStore(
			certStore,
			certEncoding,
			0,             // findFlags
			0,             // CERT_FIND_ANY
			unsafe.Pointer(nil),
			prev,
		)
		if err != nil {
			break // no more certificates
		}
		if prev != nil {
			windows.CertFreeCertificateContext(prev)
		}
		prev = cert

		// Get the subject display name
		buf := make([]uint16, 256)
		chars := windows.CertGetNameString(
			cert,
			windows.CERT_NAME_SIMPLE_DISPLAY_TYPE,
			0,
			nil,
			&buf[0],
			uint32(len(buf)),
		)
		if chars <= 1 {
			continue
		}
		name := windows.UTF16ToString(buf[:chars-1])

		// Skip CA-type certificates (they contain PCA, Root, or CA in the name)
		if containsCAMarker(name) {
			continue
		}

		// This is the signing certificate (publisher)
		if publisher == "" || len(name) > len(publisher) {
			publisher = name
		}
	}
	if prev != nil {
		windows.CertFreeCertificateContext(prev)
	}

	if publisher == "" {
		return "", fmt.Errorf("no signing certificate found in PKCS7 store")
	}

	// Step 4: Try WinVerifyTrust for signature validation.
	// Non-fatal: even if the certificate has expired, we still have the
	// publisher name and can match against the whitelist.
	fileInfo := &windows.WinTrustFileInfo{
		Size:     uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
		FilePath: pathPtr,
	}

	data := &windows.WinTrustData{
		Size:             uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:         windows.WTD_UI_NONE,
		RevocationChecks: windows.WTD_REVOKE_NONE,
		UnionChoice:      windows.WTD_CHOICE_FILE,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(fileInfo),
		StateAction: windows.WTD_STATEACTION_VERIFY,
		UIContext:   windows.WTD_UICONTEXT_EXECUTE,
	}

	verifyErr := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)

	// Always close the WinVerifyTrust state
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)

	// Log warning if verification failed, but still return the publisher name
	if verifyErr != nil {
		return publisher, fmt.Errorf("WinVerifyTrust: %w (publisher '%s' extracted anyway)", verifyErr, publisher)
	}

	return publisher, nil
}
