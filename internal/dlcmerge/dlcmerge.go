// Package dlcmerge combines several single-vehicle add-on dlcpacks into one
// dlcpack, so a large car collection costs a single dlclist.xml line instead of
// one per car. This is the way around OpenRPF 0.3's low ceiling on how many
// separate add-on dlcpacks it will mount (see docs/ROADMAP.md).
//
// Each source dlcpack is an RPF7 "OpenFormats" archive with the usual
// setup2.xml / content.xml / data / x64 layout. Merge unpacks each under a
// per-car subfolder of the combined pack and rewrites the "dlc_<car>:/" device
// token in content.xml to "dlc_<combined>:/<car>/", so nothing collides.
package dlcmerge

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/caiojohnston/gta-mod-manager/internal/rpf"
)

var (
	dataFilesRe  = regexp.MustCompile(`(?s)<dataFiles>(.*?)</dataFiles>`)
	changeSetsRe = regexp.MustCompile(`(?s)<contentChangeSets>(.*?)</contentChangeSets>`)
	changeNameRe = regexp.MustCompile(`<changeSetName>\s*([^<\s]+)\s*</changeSetName>`)
	deviceNameRe = regexp.MustCompile(`<deviceName>\s*(dlc_[^<\s]+)\s*</deviceName>`)
	nameHashRe   = regexp.MustCompile(`<nameHash>\s*([^<\s]+)\s*</nameHash>`)
)

// Merge writes a combined dlcpack into outDir (the pack's "dlc.rpf" FOLDER,
// e.g. .../mods/update/x64/dlcpacks/<combined>/dlc.rpf). combined is the pack
// name/hash (e.g. "addoncars"). carRPFs are paths to the source dlc.rpf files.
// Returns the vehicle pack names merged.
func Merge(combined string, carRPFs []string, outDir string) ([]string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	var dataFiles, changeSets []string
	var changeNames, merged []string

	for _, car := range carRPFs {
		setup, err := rpf.ReadInner(car, "setup2.xml")
		if err != nil {
			return nil, fmt.Errorf("dlcmerge: %s: reading setup2.xml: %w", filepath.Base(car), err)
		}
		content, err := rpf.ReadInner(car, "content.xml")
		if err != nil {
			return nil, fmt.Errorf("dlcmerge: %s: reading content.xml: %w", filepath.Base(car), err)
		}

		dev := firstSub(deviceNameRe, string(setup)) // "dlc_evc23yuk"
		name := firstSub(nameHashRe, string(setup))  // "evc23yuk"
		if dev == "" || name == "" {
			return nil, fmt.Errorf("dlcmerge: %s: setup2.xml missing deviceName/nameHash", filepath.Base(car))
		}

		// Unpack the car under <outDir>/<name>/, then drop its own setup/content
		// (the combined pack supplies those).
		sub := filepath.Join(outDir, name)
		// Shallow: keep the mod's nested vehicles.rpf / *_mods.rpf packed
		// (content.xml references them as RPF_FILE) so the compiled models are
		// the author's exact bytes.
		if _, err := rpf.ExtractShallow(car, sub); err != nil {
			return nil, fmt.Errorf("dlcmerge: %s: extract: %w", name, err)
		}
		os.Remove(filepath.Join(sub, "setup2.xml"))
		os.Remove(filepath.Join(sub, "content.xml"))

		// Rewrite the device token: dlc_<car>:/ -> dlc_<combined>:/<car>/
		cx := strings.ReplaceAll(string(content), dev+":/", "dlc_"+combined+":/"+name+"/")

		if m := dataFilesRe.FindStringSubmatch(cx); m != nil {
			dataFiles = append(dataFiles, strings.TrimRight(m[1], " \t\r\n"))
		}
		if m := changeSetsRe.FindStringSubmatch(cx); m != nil {
			changeSets = append(changeSets, strings.TrimRight(m[1], " \t\r\n"))
		}
		for _, cn := range changeNameRe.FindAllStringSubmatch(cx, -1) {
			changeNames = append(changeNames, cn[1])
		}
		merged = append(merged, name)
	}

	if err := os.WriteFile(filepath.Join(outDir, "content.xml"),
		[]byte(buildContentXML(dataFiles, changeSets)), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(outDir, "setup2.xml"),
		[]byte(buildSetupXML(combined, changeNames)), 0o644); err != nil {
		return nil, err
	}
	return merged, nil
}

func firstSub(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func buildContentXML(dataFiles, changeSets []string) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<CDataFileMgr__ContentsOfDataFileXml>\n")
	b.WriteString("  <disabledFiles />\n  <includedXmlFiles />\n  <includedDataFiles />\n")
	b.WriteString("  <dataFiles>")
	for _, d := range dataFiles {
		b.WriteString(d)
	}
	b.WriteString("\n  </dataFiles>\n")
	b.WriteString("  <contentChangeSets>")
	for _, c := range changeSets {
		b.WriteString(c)
	}
	b.WriteString("\n  </contentChangeSets>\n")
	b.WriteString("  <patchFiles />\n")
	b.WriteString("</CDataFileMgr__ContentsOfDataFileXml>\n")
	return b.String()
}

func buildSetupXML(combined string, changeNames []string) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<SSetupData>\n")
	fmt.Fprintf(&b, "  <deviceName>dlc_%s</deviceName>\n", combined)
	b.WriteString("  <datFile>content.xml</datFile>\n")
	b.WriteString("  <timeStamp>00/00/0000 00:00:00</timeStamp>\n")
	fmt.Fprintf(&b, "  <nameHash>%s</nameHash>\n", combined)
	b.WriteString("  <contentChangeSetGroups>\n    <Item>\n")
	b.WriteString("      <NameHash>GROUP_STARTUP</NameHash>\n      <ContentChangeSets>\n")
	for _, cn := range changeNames {
		fmt.Fprintf(&b, "        <Item>%s</Item>\n", cn)
	}
	b.WriteString("      </ContentChangeSets>\n    </Item>\n  </contentChangeSetGroups>\n")
	b.WriteString("  <type>EXTRACONTENT_COMPAT_PACK</type>\n")
	b.WriteString("  <order value=\"9\" />\n")
	b.WriteString("</SSetupData>\n")
	return b.String()
}
