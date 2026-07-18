/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

// Package signing resigns iOS applications with a user supplied signing identity
// by invoking the zsign command-line tool.
//
// zsign (https://github.com/zhlynn/zsign) is a codesign alternative for iOS that
// runs on macOS, Linux and Windows. The per-OS binaries are embedded in the GADS
// resources directory and extracted to disk on hub startup, so the hub does not
// need Xcode, the `codesign` binary or macOS to resign IPAs. Because zsign uses
// OpenSSL under the hood it accepts the same signing material as the Apple
// toolchain: either a PKCS#12 identity (+ password) or a separate certificate and
// private key.
//
// Signing assets (the identity and the .mobileprovision profile) are always
// provided by the caller - this package does not talk to App Store Connect and
// does not create certificates or profiles.
package signing

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
)

// zsign colourises its output with ANSI escape codes; strip them so the log
// reads cleanly when surfaced in the UI.
var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// Options describes a single zsign resigning job. Exactly one signing method is
// used: either P12Path (with Password), or CertPath + KeyPath (with Password for
// the private key, when it is encrypted).
type Options struct {
	// ZsignPath is the path to the extracted zsign binary for the current OS.
	ZsignPath string
	// InputIPA is the .ipa file to resign.
	InputIPA string
	// OutputIPA is where the resigned .ipa is written.
	OutputIPA string
	// ProfilePath is the .mobileprovision provisioning profile.
	ProfilePath string
	// P12Path is a PKCS#12 identity (certificate + private key). Used for the
	// p12 method - leave empty when signing with a separate cert + key.
	P12Path string
	// CertPath is a certificate (.cer/.pem). Used with KeyPath for the cert+key
	// method - leave empty when signing with a .p12.
	CertPath string
	// KeyPath is a private key (.key/.pem). Used with CertPath.
	KeyPath string
	// Password is the .p12 password, or the private key password when the key is
	// encrypted. May be empty.
	Password string
	// BundleID overrides the bundle identifier of the app. When empty the app's
	// current bundle identifier is kept.
	BundleID string
}

// BinaryName returns the embedded zsign binary filename for the current OS. The
// binaries live in the GADS resources directory and are extracted to disk on hub
// startup.
func BinaryName() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "zsign-mac", nil
	case "linux":
		return "zsign-linux", nil
	case "windows":
		return "zsign-win.exe", nil
	default:
		return "", fmt.Errorf("iOS signing is not supported on %s hosts", runtime.GOOS)
	}
}

// Sign resigns opts.InputIPA into opts.OutputIPA with zsign and returns zsign's
// combined stdout/stderr output along with any execution error. A non-nil error
// means signing failed; the returned log carries zsign's own diagnostics.
func Sign(opts Options) (string, error) {
	if opts.ZsignPath == "" {
		return "", fmt.Errorf("zsign binary path is required")
	}
	if opts.InputIPA == "" || opts.OutputIPA == "" {
		return "", fmt.Errorf("input and output ipa paths are required")
	}
	if opts.ProfilePath == "" {
		return "", fmt.Errorf("provisioning profile is required")
	}

	var args []string
	switch {
	case opts.P12Path != "":
		args = append(args, "-k", opts.P12Path)
	case opts.CertPath != "" && opts.KeyPath != "":
		args = append(args, "-c", opts.CertPath, "-k", opts.KeyPath)
	default:
		return "", fmt.Errorf("no signing material provided - supply a .p12 or a certificate and private key")
	}
	if opts.Password != "" {
		args = append(args, "-p", opts.Password)
	}
	// -z 9 = maximum zip compression for the output ipa (it is stored in GridFS).
	args = append(args, "-m", opts.ProfilePath, "-z", "9", "-o", opts.OutputIPA)
	if opts.BundleID != "" {
		args = append(args, "-b", opts.BundleID)
	}
	// The input ipa is the final positional argument.
	args = append(args, opts.InputIPA)

	cmd := exec.Command(opts.ZsignPath, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return ansiEscape.ReplaceAllString(out.String(), ""), err
}
