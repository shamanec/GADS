/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

package router

import (
	"GADS/common/api"
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
)

type generateCSRRequest struct {
	CommonName   string `json:"common_name"`
	Email        string `json:"email"`
	Country      string `json:"country"`
	Organization string `json:"organization"`
}

// GenerateCSR godoc
// @Summary      Generate a signing CSR
// @Description  Build a fresh 2048-bit RSA private key and a matching Certificate Signing Request from the supplied subject fields, returned as a zip (CSR + private key + README). The hub does not persist any of the generated material - once the response is flushed the key only exists on the client. Lets users without a Mac obtain an Apple signing certificate.
// @Tags         Hub - Admin - Files
// @Accept       json
// @Produce      application/zip
// @Param        request  body      object  true  "CSR subject fields (common_name, email, country, organization)"
// @Success      200      {file}    binary
// @Failure      400      {object}  models.ErrorResponse
// @Failure      500      {object}  models.ErrorResponse
// @Security     BearerAuth
// @Router       /admin/files/csr [post]
func GenerateCSR(c *gin.Context) {
	var req generateCSRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "Invalid request body")
		return
	}

	commonName := strings.TrimSpace(req.CommonName)
	email := strings.TrimSpace(req.Email)
	country := strings.ToUpper(strings.TrimSpace(req.Country))
	organization := strings.TrimSpace(req.Organization)

	if commonName == "" || len(commonName) > 64 {
		api.BadRequest(c, "Common name is required (max 64 characters)")
		return
	}
	if _, err := mail.ParseAddress(email); err != nil {
		api.BadRequest(c, "A valid email address is required")
		return
	}
	if len(country) != 2 || !isAllLetters(country) {
		api.BadRequest(c, "Country must be a 2-letter ISO code (e.g. US, DE, GB)")
		return
	}
	if len(organization) > 64 {
		api.BadRequest(c, "Organization must be 64 characters or fewer")
		return
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		api.InternalError(c, "Failed to generate private key")
		return
	}

	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: commonName,
			Country:    []string{country},
		},
		EmailAddresses:     []string{email},
		SignatureAlgorithm: x509.SHA256WithRSA,
	}
	if organization != "" {
		template.Subject.Organization = []string{organization}
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, privateKey)
	if err != nil {
		api.InternalError(c, "Failed to create CSR")
		return
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	zipBytes, err := buildCSRZip(csrPEM, keyPEM, commonName, email)
	if err != nil {
		api.InternalError(c, "Failed to package files")
		return
	}

	filename := fmt.Sprintf(
		"csr-%s-%s.zip",
		sanitizeForFilename(commonName),
		time.Now().UTC().Format("20060102-150405"),
	)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/zip", zipBytes)
}

func buildCSRZip(csrPEM, keyPEM []byte, commonName, email string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	if err := writeZipEntry(zw, "request.certSigningRequest", csrPEM); err != nil {
		return nil, err
	}
	if err := writeZipEntry(zw, "private.key", keyPEM); err != nil {
		return nil, err
	}
	if err := writeZipEntry(zw, "README.txt", []byte(csrReadme(commonName, email))); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeZipEntry(zw *zip.Writer, name string, content []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(content)
	return err
}

func csrReadme(commonName, email string) string {
	return fmt.Sprintf(`GADS - Certificate Signing Request
==================================

Generated for:
  Common Name : %s
  Email       : %s

This archive contains:

  request.certSigningRequest
    The CSR you upload to the Apple Developer Portal when creating a
    new signing certificate (Certificates, Identifiers & Profiles >
    Certificates > "+" > select certificate type > "Choose File").

  private.key
    The RSA private key matched to the CSR above. Keep this file safe.
    Apple does NOT receive it. You will need it later, together with
    the certificate Apple issues, to sign WebDriverAgent IPAs.

IMPORTANT
---------
GADS does not store a copy of the private key. If you lose this file
you will need to generate a new CSR, register it on the Apple Developer
Portal, and reissue the certificate.

Using the cert with GADS
------------------------
Once Apple issues the .cer, go to Admin > Files > WebDriverAgent >
"Upload & sign", choose "Certificate + key" as the signing material,
and upload the .cer alongside the private.key from this archive.
No conversion needed.

If you would rather bundle the cert and key into a portable .p12 (to
use the .p12 signing path instead), the standard openssl one-liner is:

  openssl x509 -inform DER -in apple.cer -out apple.pem
  openssl pkcs12 -export \
    -inkey private.key \
    -in apple.pem \
    -out signing.p12

The password you set is the one you will type when uploading the .p12.
`, commonName, email)
}

func sanitizeForFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		case r == ' ', r == '-', r == '_':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "csr"
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func isAllLetters(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}
