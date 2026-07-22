package usecase

import (
	"fmt"
	"regexp"
	"strings"
)

func admsEscapeValue(value string) string {
	escaped := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(strings.TrimSpace(value))
	return escaped
}

func normalizeADMSPin(pin string) string {
	normalized := strings.TrimSpace(pin)
	if normalized == "" {
		return ""
	}

	prefixPattern := regexp.MustCompile(`(?i)^EMP[-_ ]?`)
	normalized = prefixPattern.ReplaceAllString(normalized, "")
	if normalized == "" {
		return ""
	}
	return normalized
}

func buildADMSUserCommand(pin, fullName, cardNo string) string {
	variants := buildADMSUserCommandVariants(pin, fullName, cardNo)
	if len(variants) == 0 {
		return ""
	}
	return variants[0]
}

func buildADMSUserCommandVariants(pin, fullName, cardNo string) []string {
	normalizedPin := normalizeADMSPin(pin)
	fullName = admsEscapeValue(fullName)
	cardNo = admsEscapeValue(cardNo)

	return []string{
		fmt.Sprintf("DATA UPDATE USERINFO PIN=%s\tNAME=%s\tPRI=0", normalizedPin, fullName),
		fmt.Sprintf("DATA UPDATE USERINFO Pin=%s\tName=%s\tPri=0", normalizedPin, fullName),
		fmt.Sprintf("DATA UPDATE USER PIN=%s\tName=%s\tPri=0", normalizedPin, fullName),
		fmt.Sprintf("DATA UPDATE USER Pin=%s\tName=%s\tPri=0", normalizedPin, fullName),
		fmt.Sprintf("DATA UPDATE user PIN=%s\tName=%s\tPri=0", normalizedPin, fullName),
	}
}

func nextADMSUserCommandVariant(pin, fullName, cardNo, current string) string {
	variants := buildADMSUserCommandVariants(pin, fullName, cardNo)
	for i, variant := range variants {
		if strings.EqualFold(strings.TrimSpace(variant), strings.TrimSpace(current)) {
			if i+1 < len(variants) {
				return variants[i+1]
			}
			break
		}
	}
	return ""
}

type admsUserPayload struct {
	Pin      string
	FullName string
	CardNo   string
}

func parseADMSUserPayload(command string) *admsUserPayload {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return nil
	}

	payload := &admsUserPayload{}
	parts := strings.Split(trimmed, "\t")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(kv[0]))
		value := strings.TrimSpace(kv[1])
		switch key {
		case "PIN":
			payload.Pin = value
		case "NAME":
			payload.FullName = value
		case "CARD":
			payload.CardNo = value
		}
	}

	if payload.Pin == "" && payload.FullName == "" && payload.CardNo == "" {
		return nil
	}
	return payload
}

func buildADMSEnrollCommand(pin string) string {
	normalizedPin := normalizeADMSPin(pin)
	return fmt.Sprintf("ENROLL_FP PIN=%s\tFID=0\tRETRY=3\tOVERWRITE=1", admsEscapeValue(normalizedPin))
}

func buildADMSFallbackEnrollCommand(pin string) string {
	normalizedPin := normalizeADMSPin(pin)
	return fmt.Sprintf("ENROLL_FP PIN=%s\tFID=0\tRETRY=3\tOVERWRITE=1", admsEscapeValue(normalizedPin))
}

func buildADMSFingerprintCommand(pin string, fingerIndex int, size int, template string) string {
	variants := buildADMSFingerprintCommandVariants(pin, fingerIndex, size, template)
	if len(variants) == 0 {
		return ""
	}
	return variants[0]
}

func buildADMSFingerprintCommandVariants(pin string, fingerIndex int, size int, template string) []string {
	normalizedPin := normalizeADMSPin(pin)
	template = admsEscapeValue(template)
	return []string{
		fmt.Sprintf("DATA UPDATE BIODATA PIN=%s\tNo=1\tIndex=%d\tValid=1\tDuress=0\tType=9\tMajorVer=5\tMinorVer=8\tFormat=0\tTmp=%s", normalizedPin, fingerIndex, template),
		fmt.Sprintf("DATA UPDATE FINGERTEMPLATE PIN=%s\tFID=%d\tSIZE=%d\tVALID=1\tTMP=%s", normalizedPin, fingerIndex, size, template),
		fmt.Sprintf("DATA UPDATE TEMPLATEV10 PIN=%s\tFID=%d\tSIZE=%d\tVALID=1\tTMP=%s", normalizedPin, fingerIndex, size, template),
		fmt.Sprintf("DATA UPDATE TEMPLATEV10 Pin=%s\tFID=%d\tSize=%d\tValid=1\tTmp=%s", normalizedPin, fingerIndex, size, template),
		fmt.Sprintf("DATA UPDATE TEMPLATEV10 PIN=%s\tFingerID=%d\tSize=%d\tVal=1\tTemplate=%s", normalizedPin, fingerIndex, size, template),
		fmt.Sprintf("DATA UPDATE FINGERTEMPLATE PIN=%s\tFingerID=%d\tSize=%d\tVal=1\tTemplate=%s", normalizedPin, fingerIndex, size, template),
	}
}

func nextADMSFingerprintCommandVariant(pin string, fingerIndex int, size int, template, current string) string {
	variants := buildADMSFingerprintCommandVariants(pin, fingerIndex, size, template)
	for i, variant := range variants {
		if strings.EqualFold(strings.TrimSpace(variant), strings.TrimSpace(current)) {
			if i+1 < len(variants) {
				return variants[i+1]
			}
			break
		}
	}
	return ""
}
