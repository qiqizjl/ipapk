package ipapk

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/andrianbdn/iospng"
	"github.com/shogo82148/androidbinary"
	"github.com/shogo82148/androidbinary/apk"
	"howett.net/plist"
)

var (
	reInfoPlist          = regexp.MustCompile(`Payload/[^/]+/Info\.plist`)
	reMobileProvision    = regexp.MustCompile(`Payload/[^/]+/embedded\.mobileprovision`)
	ErrNoIcon            = errors.New("icon not found")
	ErrNoMobileProvision = errors.New("mobileprovision not found")
)

const (
	iosExt          = ".ipa"
	androidExt      = ".apk"
	PlatformIOS     = 1
	PlatformAndroid = 2
	IOSAdHoc        = 1
	IOSAppStore     = 2
	IOSEnterprise   = 3
)

type AppInfo struct {
	Name     string
	BundleId string
	Version  string
	Build    string
	Icon     image.Image
	Size     int64
	Platform int
	IOS      iosInfo
}

type iosInfo struct {
	Type        int
	TeamName    string
	AllowDevice []string
}

type androidManifest struct {
	Package     string `xml:"package,attr"`
	VersionName string `xml:"versionName,attr"`
	VersionCode string `xml:"versionCode,attr"`
}

type iosPlist struct {
	CFBundleName         string `plist:"CFBundleName"`
	CFBundleDisplayName  string `plist:"CFBundleDisplayName"`
	CFBundleVersion      string `plist:"CFBundleVersion"`
	CFBundleShortVersion string `plist:"CFBundleShortVersionString"`
	CFBundleIdentifier   string `plist:"CFBundleIdentifier"`
}

type iosMobileProvision struct {
	ProvisionsAllDevices *bool     `plist:"ProvisionsAllDevices"`
	TeamName             string    `plist:"TeamName"`
	ProvisionedDevices   *[]string `plist:"ProvisionedDevices"`
}

func NewAppParser(name string) (*AppInfo, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(file, stat.Size())
	if err != nil {
		return nil, err
	}

	var xmlFile, plistFile, iosIconFile, iosMobileProvisionFile *zip.File
	for _, f := range reader.File {
		switch {
		case f.Name == "AndroidManifest.xml":
			xmlFile = f
		case reInfoPlist.MatchString(f.Name):
			plistFile = f
		case reMobileProvision.MatchString(f.Name):
			iosMobileProvisionFile = f
		case strings.Contains(f.Name, "AppIcon60x60"):
			iosIconFile = f
		}
	}

	ext := filepath.Ext(stat.Name())

	if ext == androidExt {
		info, err := parseApkFile(xmlFile)
		icon, label, err := parseApkIconAndLabel(name)
		info.Name = label
		info.Icon = icon
		info.Size = stat.Size()
		return info, err
	}

	if ext == iosExt {
		info, err := parseIpaFile(plistFile)
		icon, err := parseIpaIcon(iosIconFile)
		info.Icon = icon
		info.Size = stat.Size()
		iosInfoData, _ := getIosInfo(iosMobileProvisionFile)
		if iosInfoData != nil {
			info.IOS = *iosInfoData
		}
		//getIosInfo
		return info, err
	}

	return nil, errors.New("unknown platform")
}

func parseAndroidManifest(xmlFile *zip.File) (*androidManifest, error) {
	rc, err := xmlFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	buf, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	xmlContent, err := androidbinary.NewXMLFile(bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	manifest := new(androidManifest)
	decoder := xml.NewDecoder(xmlContent.Reader())
	if err := decoder.Decode(manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func parseApkFile(xmlFile *zip.File) (*AppInfo, error) {
	if xmlFile == nil {
		return nil, errors.New("AndroidManifest.xml not found")
	}

	manifest, err := parseAndroidManifest(xmlFile)
	if err != nil {
		return nil, err
	}

	info := new(AppInfo)
	info.BundleId = manifest.Package
	info.Version = manifest.VersionName
	info.Build = manifest.VersionCode
	info.Platform = PlatformAndroid

	return info, nil
}

func parseApkIconAndLabel(name string) (image.Image, string, error) {
	pkg, err := apk.OpenFile(name)
	if err != nil {
		return nil, "", err
	}
	defer pkg.Close()

	icon, _ := pkg.Icon(&androidbinary.ResTableConfig{
		Density: 720,
	})
	if icon == nil {
		return nil, "", ErrNoIcon
	}

	label, _ := pkg.Label(nil)

	return icon, label, nil
}

func parseIpaFile(plistFile *zip.File) (*AppInfo, error) {
	if plistFile == nil {
		return nil, errors.New("info.plist not found")
	}

	rc, err := plistFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	buf, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	p := new(iosPlist)
	decoder := plist.NewDecoder(bytes.NewReader(buf))
	if err := decoder.Decode(p); err != nil {
		return nil, err
	}

	info := new(AppInfo)
	if p.CFBundleDisplayName == "" {
		info.Name = p.CFBundleName
	} else {
		info.Name = p.CFBundleDisplayName
	}
	info.BundleId = p.CFBundleIdentifier
	info.Version = p.CFBundleShortVersion
	info.Build = p.CFBundleVersion
	info.Platform = PlatformIOS

	return info, nil
}

func parseIpaIcon(iconFile *zip.File) (image.Image, error) {
	if iconFile == nil {
		return nil, ErrNoIcon
	}

	rc, err := iconFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var w bytes.Buffer
	iospng.PngRevertOptimization(rc, &w)

	return png.Decode(bytes.NewReader(w.Bytes()))
}

func getIosInfo(file *zip.File) (*iosInfo, error) {
	info, err := parseIpaMobileProvision(file)
	if err != nil {
		return nil, err
	}
	result := iosInfo{}
	result.TeamName = info.TeamName
	result.Type = IOSAppStore
	result.AllowDevice = make([]string, 0)
	if info.ProvisionedDevices != nil {
		result.Type = IOSAdHoc
		result.AllowDevice = *info.ProvisionedDevices
	}
	if info.ProvisionsAllDevices != nil {
		result.Type = IOSEnterprise
	}
	return &result, nil
}

func parseIpaMobileProvision(file *zip.File) (*iosMobileProvision, error) {
	if file == nil {
		return nil, ErrNoMobileProvision
	}
	localFile := "/tmp/" + makeMD5(file.Name+time.Now().Format("2006-01-02 15:04:05")) + ".mobileprovision"
	f, err := os.Create(localFile)
	if err != nil {
		return nil, err
	}
	defer os.Remove(localFile)
	res, err := file.Open()
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(f, res)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("openssl", "smime", "-inform", "der", "-verify", "-noverify", "-in", localFile)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	p := new(iosMobileProvision)
	decoder := plist.NewDecoder(bytes.NewReader(out))
	if err := decoder.Decode(p); err != nil {
		return nil, err
	}
	return p, nil
}

func makeMD5(text string) string {
	ctx := md5.New()
	ctx.Write([]byte(text))
	return hex.EncodeToString(ctx.Sum(nil))
}
