package dlcmerge

import (
	"strings"
	"testing"
)

const sampleContent = `<?xml version="1.0" encoding="UTF-8"?>
<CDataFileMgr__ContentsOfDataFileXml>
  <dataFiles>
    <Item>
      <filename>dlc_carx:/data/vehicles.meta</filename>
      <fileType>VEHICLE_METADATA_FILE</fileType>
    </Item>
    <Item>
      <filename>dlc_carx:/%PLATFORM%/vehicles.rpf</filename>
      <fileType>RPF_FILE</fileType>
    </Item>
  </dataFiles>
  <contentChangeSets>
    <Item>
      <changeSetName>carx_AUTOGEN</changeSetName>
      <filesToEnable>
        <Item>dlc_carx:/data/vehicles.meta</Item>
      </filesToEnable>
    </Item>
  </contentChangeSets>
</CDataFileMgr__ContentsOfDataFileXml>`

const sampleSetup = `<SSetupData>
  <deviceName>dlc_carx</deviceName>
  <nameHash>carx</nameHash>
</SSetupData>`

func TestFirstSubAndRewrite(t *testing.T) {
	dev := firstSub(deviceNameRe, sampleSetup)
	name := firstSub(nameHashRe, sampleSetup)
	if dev != "dlc_carx" || name != "carx" {
		t.Fatalf("parsed device=%q name=%q", dev, name)
	}

	cx := strings.ReplaceAll(sampleContent, dev+":/", "dlc_addoncars:/"+name+"/")
	if strings.Contains(cx, "dlc_carx:/") {
		t.Error("device token not fully rewritten")
	}
	if !strings.Contains(cx, "dlc_addoncars:/carx/data/vehicles.meta") ||
		!strings.Contains(cx, "dlc_addoncars:/carx/%PLATFORM%/vehicles.rpf") {
		t.Errorf("rewritten paths wrong:\n%s", cx)
	}

	df := dataFilesRe.FindStringSubmatch(cx)
	cs := changeSetsRe.FindStringSubmatch(cx)
	if df == nil || cs == nil {
		t.Fatal("dataFiles / contentChangeSets not extracted")
	}
	names := changeNameRe.FindAllStringSubmatch(cx, -1)
	if len(names) != 1 || names[0][1] != "carx_AUTOGEN" {
		t.Errorf("changeset names = %v", names)
	}
}

func TestBuildXML(t *testing.T) {
	c := buildContentXML([]string{"\n  <Item>A</Item>", "\n  <Item>B</Item>"},
		[]string{"\n  <Item>cs1</Item>"})
	if !strings.Contains(c, "<Item>A</Item>") || !strings.Contains(c, "<Item>B</Item>") ||
		!strings.Contains(c, "<Item>cs1</Item>") {
		t.Errorf("buildContentXML missing pieces:\n%s", c)
	}
	if !strings.Contains(c, "</dataFiles>") || !strings.Contains(c, "</contentChangeSets>") {
		t.Error("buildContentXML not closed")
	}

	s := buildSetupXML("addoncars", []string{"carx_AUTOGEN", "cary_AUTOGEN"})
	if !strings.Contains(s, "<deviceName>dlc_addoncars</deviceName>") ||
		!strings.Contains(s, "<nameHash>addoncars</nameHash>") ||
		!strings.Contains(s, "<Item>carx_AUTOGEN</Item>") ||
		!strings.Contains(s, "<Item>cary_AUTOGEN</Item>") {
		t.Errorf("buildSetupXML wrong:\n%s", s)
	}
}
