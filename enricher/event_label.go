package enricher

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"strings"
)

var eventCatalogNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// EventCatalogTag returns the registry key used for event type rows (matches gedcom-go / DB seeds).
func EventCatalogTag(eventType, customType string) string {
	et := strings.TrimSpace(eventType)
	ct := strings.TrimSpace(customType)
	if strings.EqualFold(et, "EVEN") && ct != "" {
		slug := strings.Trim(eventCatalogNonAlnum.ReplaceAllString(strings.ToLower(ct), "_"), "_")
		if slug == "" {
			slug = "unknown"
		}
		sum := md5.Sum([]byte(strings.ToLower(ct)))
		md8 := hex.EncodeToString(sum[:])[:8]
		return "EVEN__" + slug + "_" + md8
	}
	return strings.ToUpper(et)
}

type eventStandardMeta struct {
	label string
}

// standardEventCatalog keys are GEDCOM tags (uppercase).
var standardEventCatalog = map[string]eventStandardMeta{
	"ADOP": {"Adoption"}, "ANUL": {"Annulment"}, "BAPM": {"Baptism"}, "BARM": {"Bar Mitzvah"},
	"BASM": {"Bat Mitzvah"}, "BAPL": {"LDS Baptism"}, "BIRT": {"Birth"}, "BLES": {"Blessing"},
	"BURI": {"Burial"}, "CAST": {"Caste"}, "CENS": {"Census"}, "CHR": {"Christening"},
	"CHRA": {"Adult Christening"}, "CONF": {"Confirmation"}, "CREM": {"Cremation"},
	"DEAT": {"Death"}, "DIV": {"Divorce"}, "DIVF": {"Divorce Filed"}, "DSCR": {"Description"},
	"EDUC": {"Education"}, "EMIG": {"Emigration"}, "ENGA": {"Engagement"}, "EVEN": {"Event"},
	"FACT": {"Fact"}, "FCOM": {"First Communion"}, "GRAD": {"Graduation"}, "IDNO": {"National ID"},
	"IMMI": {"Immigration"}, "MARB": {"Marriage Bann"}, "MARC": {"Marriage Contract"},
	"MARL": {"Marriage License"}, "MARR": {"Marriage"}, "MARS": {"Marriage Settlement"},
	"NATI": {"Nationality"}, "NATU": {"Naturalization"}, "NCHI": {"Children Count"},
	"NMR": {"Marriage Count"}, "OCCU": {"Occupation"}, "ORDN": {"Ordination"},
	"PROB": {"Probate"}, "PROP": {"Property"}, "RELI": {"Religion"}, "RESI": {"Residence"},
	"RETI": {"Retirement"}, "SSN": {"Social Security Number"}, "TITL": {"Title"}, "WILL": {"Will"},
}

// EventLabelFor returns a human-readable label for gedcom_events_v2.event_label.
func EventLabelFor(eventType, customType string) string {
	tag := EventCatalogTag(eventType, customType)
	if m, ok := standardEventCatalog[strings.ToUpper(strings.TrimSpace(tag))]; ok {
		return m.label
	}
	if strings.HasPrefix(tag, "EVEN__") {
		ct := strings.TrimSpace(customType)
		if ct != "" {
			return ct
		}
		return tag
	}
	if m, ok := standardEventCatalog[strings.ToUpper(strings.TrimSpace(eventType))]; ok {
		return m.label
	}
	return tag
}
