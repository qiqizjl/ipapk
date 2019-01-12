package ipapk

import (
	"archive/zip"
	"bytes"
	"image/png"
	"os"
	"strings"
	"testing"
)

func getAppZipReader(filename string) (*zip.Reader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(file, stat.Size())
	if err != nil {
		return nil, err
	}
	return reader, nil
}

func getAndroidManifest() (*zip.File, error) {
	reader, err := getAppZipReader("testdata/helloworld.apk")
	if err != nil {
		return nil, err
	}
	var xmlFile *zip.File
	for _, f := range reader.File {
		if f.Name == "AndroidManifest.xml" {
			xmlFile = f
			break
		}
	}
	return xmlFile, nil
}

func TestParseAndroidManifest(t *testing.T) {
	xmlFile, err := getAndroidManifest()
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	manifest, err := parseAndroidManifest(xmlFile)
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	if manifest.Package != "com.example.helloworld" {
		t.Errorf("got %v want %v", manifest.Package, "com.example.helloworld")
	}
	if manifest.VersionName != "1.0" {
		t.Errorf("got %v want %v", manifest.VersionName, "1.0")
	}
	if manifest.VersionCode != "1" {
		t.Errorf("got %v want %v", manifest.VersionCode, "1")
	}
}

func TestParseApkFile(t *testing.T) {
	xmlFile, err := getAndroidManifest()
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	apk, err := parseApkFile(xmlFile)
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	if apk.BundleId != "com.example.helloworld" {
		t.Errorf("got %v want %v", apk.BundleId, "com.example.helloworld")
	}
	if apk.Version != "1.0" {
		t.Errorf("got %v want %v", apk.Version, "1.0")
	}
	if apk.Build != "1" {
		t.Errorf("got %v want %v", apk.Build, "1")
	}
	if apk.Platform != PlatformAndroid {
		t.Errorf("got %d want %d", apk.Platform, PlatformAndroid)
	}
}

func TestParseApkIconAndLabel(t *testing.T) {
	icon, label, err := parseApkIconAndLabel("testdata/helloworld.apk")
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	buf := new(bytes.Buffer)
	if err := png.Encode(buf, icon); err != nil {
		t.Errorf("got %v want no error", err)
	}
	if len(buf.Bytes()) != 10223 {
		t.Errorf("got %v want %v", len(buf.Bytes()), 10223)
	}
	if label != "HelloWorld" {
		t.Errorf("got %v want %v", label, "HelloWorld")
	}
}

func getIosPlist() (*zip.File, error) {
	reader, err := getAppZipReader("testdata/helloworld.ipa")
	if err != nil {
		return nil, err
	}
	var plistFile *zip.File
	for _, f := range reader.File {
		if reInfoPlist.MatchString(f.Name) {
			plistFile = f
			break
		}
	}
	return plistFile, nil
}

func getIosMobileProvision() (*zip.File, error) {
	reader, err := getAppZipReader("testdata/helloworld.ipa")
	if err != nil {
		return nil, err
	}
	var mobileProvisionFile *zip.File
	for _, f := range reader.File {
		if reMobileProvision.MatchString(f.Name) {
			mobileProvisionFile = f
			break
		}
	}
	return mobileProvisionFile, nil
}

func TestParseIpaFile(t *testing.T) {
	plistFile, err := getIosPlist()
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	ipa, err := parseIpaFile(plistFile)
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	if ipa.BundleId != "com.kthcorp.helloworld" {
		t.Errorf("got %v want %v", ipa.BundleId, "com.kthcorp.helloworld")
	}
	if ipa.Version != "1.0" {
		t.Errorf("got %v want %v", ipa.Version, "1.0")
	}
	if ipa.Build != "1.0" {
		t.Errorf("got %v want %v", ipa.Build, "1.0")
	}
	if ipa.Platform != PlatformIOS {
		t.Errorf("got %d want %d", ipa.Platform, PlatformIOS)
	}
}

func TestParseIpaMobileProvision(t *testing.T) {
	mobileProvisionFile, err := getIosMobileProvision()
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	info, err := getIosInfo(mobileProvisionFile)
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	if info.Type != IOSEnterprise {
		t.Errorf("got %d want %d", info.Type, IOSEnterprise)
	}
}

func TestParseIpaIcon(t *testing.T) {
	reader, err := getAppZipReader("testdata/helloworld.ipa")
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	var iconFile *zip.File
	for _, f := range reader.File {
		if strings.Contains(f.Name, "AppIcon60x60") {
			iconFile = f
			break
		}
	}
	if _, err := parseIpaIcon(iconFile); err != ErrNoIcon {
		t.Errorf("got %v want %v", err, ErrNoIcon)
	}
}
