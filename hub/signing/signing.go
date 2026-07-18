/*
 * This file is part of GADS.
 *
 * Copyright (c) 2022-2025 Nikola Shabanov
 *
 * This source code is licensed under the GNU Affero General Public License v3.0.
 * You may obtain a copy of the license at https://www.gnu.org/licenses/agpl-3.0.html
 */

// Package signing resigns iOS applications with a user supplied signing identity.
//
// The signing itself is done in pure Go by github.com/aluedeke/go-codesign, so the
// hub does not need Xcode, the `codesign` binary or even macOS to run this.
//
// Signing assets (the .p12 identity and the .mobileprovision profile) are always
// provided by the caller - this package does not talk to App Store Connect and
// does not create certificates or profiles.
package signing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aluedeke/go-codesign/pkg/codesign"
)

// SignOptions describes a single resigning job.
type SignOptions struct {
	// AppPath is the .ipa file or .app bundle folder to resign.
	AppPath string
	// OutputPath is where the resigned app is written. When empty a sibling path
	// with a `-signed` suffix is used.
	OutputPath string
	// BundleID overrides the bundle identifier of the app. When empty the current
	// bundle identifier of the app is kept.
	BundleID string
	// P12Data is the raw PKCS#12 signing identity (certificate + private key).
	P12Data []byte
	// P12Password is the password protecting P12Data.
	P12Password string
	// ProfileData is the raw .mobileprovision provisioning profile.
	ProfileData []byte
}

// SignResult describes the outcome of a resigning job.
type SignResult struct {
	// OutputPath is the path of the resigned app.
	OutputPath string
	// BundleID is the bundle identifier the app was signed with.
	BundleID string
}

// Sign resigns the app at opts.AppPath with the provided identity and profile.
//
// Both .ipa files and .app bundle folders are supported. The input is never
// modified - the app is unpacked/copied into a temporary directory, resigned
// there and then written to opts.OutputPath.
func Sign(opts SignOptions) (SignResult, error) {
	if opts.AppPath == "" {
		return SignResult{}, fmt.Errorf("app path is required")
	}
	if len(opts.P12Data) == 0 {
		return SignResult{}, fmt.Errorf("p12 signing identity is required")
	}
	if len(opts.ProfileData) == 0 {
		return SignResult{}, fmt.Errorf("provisioning profile is required")
	}
	if opts.OutputPath == "" {
		opts.OutputPath = defaultOutputPath(opts.AppPath)
	}
	if opts.BundleID == "" {
		bundleID, err := GetBundleID(opts.AppPath)
		if err != nil {
			return SignResult{}, err
		}
		opts.BundleID = bundleID
	}

	isIPA := strings.EqualFold(filepath.Ext(opts.AppPath), ".ipa")

	// Unpack or copy the app into a temp dir so the input is left untouched,
	// `codesign.Resign` works in place.
	var workDir string
	var appPath string
	if isIPA {
		var err error
		workDir, err = codesign.ExtractIPA(opts.AppPath)
		if err != nil {
			return SignResult{}, fmt.Errorf("failed extracting ipa - %s", err)
		}
		defer os.RemoveAll(workDir)

		appPath, err = codesign.FindAppBundle(workDir)
		if err != nil {
			return SignResult{}, fmt.Errorf("failed finding app bundle in ipa - %s", err)
		}
	} else {
		var err error
		workDir, err = os.MkdirTemp("", "gads-signing-*")
		if err != nil {
			return SignResult{}, fmt.Errorf("failed creating temp dir for signing - %s", err)
		}
		defer os.RemoveAll(workDir)

		appPath = filepath.Join(workDir, filepath.Base(opts.AppPath))
		if err := codesign.CopyAppBundle(opts.AppPath, appPath); err != nil {
			return SignResult{}, fmt.Errorf("failed copying app bundle - %s", err)
		}
	}

	err := codesign.Resign(codesign.ResignOptions{
		AppPath:             appPath,
		P12Data:             opts.P12Data,
		P12Password:         opts.P12Password,
		ProvisioningProfile: opts.ProfileData,
		NewBundleID:         opts.BundleID,
	})
	if err != nil {
		return SignResult{}, fmt.Errorf("failed signing app - %s", err)
	}

	if isIPA {
		if err := codesign.RepackageIPA(workDir, opts.OutputPath); err != nil {
			return SignResult{}, fmt.Errorf("failed repackaging ipa - %s", err)
		}
	} else {
		if err := os.RemoveAll(opts.OutputPath); err != nil && !os.IsNotExist(err) {
			return SignResult{}, fmt.Errorf("failed removing existing output app bundle - %s", err)
		}
		if err := codesign.CopyAppBundle(appPath, opts.OutputPath); err != nil {
			return SignResult{}, fmt.Errorf("failed writing signed app bundle - %s", err)
		}
	}

	return SignResult{OutputPath: opts.OutputPath, BundleID: opts.BundleID}, nil
}

// SignFiles resigns the app at appPath using a .p12 and .mobileprovision read
// from disk. Convenience wrapper around Sign for when the assets were downloaded
// to the hub filesystem instead of being held in memory.
func SignFiles(appPath, outputPath, bundleID, p12Path, p12Password, profilePath string) (SignResult, error) {
	if p12Path == "" {
		return SignResult{}, fmt.Errorf("p12 path is required")
	}
	if profilePath == "" {
		return SignResult{}, fmt.Errorf("provisioning profile path is required")
	}

	p12Data, err := os.ReadFile(p12Path)
	if err != nil {
		return SignResult{}, fmt.Errorf("failed reading p12 identity - %s", err)
	}
	profileData, err := os.ReadFile(profilePath)
	if err != nil {
		return SignResult{}, fmt.Errorf("failed reading provisioning profile - %s", err)
	}

	return Sign(SignOptions{
		AppPath:     appPath,
		OutputPath:  outputPath,
		BundleID:    bundleID,
		P12Data:     p12Data,
		P12Password: p12Password,
		ProfileData: profileData,
	})
}

// GetBundleID returns the current bundle identifier of an .ipa file or .app
// bundle folder.
func GetBundleID(appPath string) (string, error) {
	if !strings.EqualFold(filepath.Ext(appPath), ".ipa") {
		return codesign.GetAppBundleID(appPath)
	}

	tempDir, err := codesign.ExtractIPA(appPath)
	if err != nil {
		return "", fmt.Errorf("failed extracting ipa - %s", err)
	}
	defer os.RemoveAll(tempDir)

	bundlePath, err := codesign.FindAppBundle(tempDir)
	if err != nil {
		return "", fmt.Errorf("failed finding app bundle in ipa - %s", err)
	}
	return codesign.GetAppBundleID(bundlePath)
}

// defaultOutputPath returns a sibling path of appPath with a `-signed` suffix
// added before the extension.
func defaultOutputPath(appPath string) string {
	ext := filepath.Ext(appPath)
	if ext == "" {
		return appPath + "-signed"
	}
	return strings.TrimSuffix(appPath, ext) + "-signed" + ext
}
