package queryutil

import "strings"

var jsonPathReplacer = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

func JSONPath(parts []string) string {
	path := "$"
	for _, part := range parts {
		if isJSONPathIdentifier(part) {
			path += "." + part
			continue
		}
		path += `."` + jsonPathReplacer.Replace(part) + `"`
	}
	return path
}

func isJSONPathIdentifier(part string) bool {
	if part == "" {
		return false
	}
	for index, char := range part {
		if char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' {
			continue
		}
		if index > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}
