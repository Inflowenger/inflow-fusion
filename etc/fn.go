package etc

import (
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/google/uuid"
)



func ReplaceByMapString(text string, replacements map[string]any) string {

	// Create a regular expression to find placeholders like {key}
	re := regexp.MustCompile(`{\s*\w+\s*}`)

	// Use ReplaceAllStringFunc to replace each match
	result := re.ReplaceAllStringFunc(text, func(match string) string {
		// Extract the key from the match (e.g., "name" from "{name}")
		key := strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")
		key = strings.Trim(key, " ")
		// Look up the key in the replacements map
		if val, ok := replacements[key]; ok {
			return fmt.Sprintf("%v", val)
		}
		// If key not found in map, return the original placeholder
		return match
	})
	return result
}
func ReplaceAllWith(text ,replaceChar string) string {

	// Create a regular expression to find placeholders like {key}
	re := regexp.MustCompile(`{\s*\w+\s*}`)

	// Use ReplaceAllStringFunc to replace each match
	result := re.ReplaceAllStringFunc(text, func(match string) string {
		// Extract the key from the match (e.g., "name" from "{name}")
		key := strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")
		key = strings.Trim(key, " ")
		// Look up the key in the replacements map
		// if val, ok := replacements[key]; ok {
			return fmt.Sprintf("%v", replaceChar)
		// }

	})
	return result
}
func UUID()string{
	return uuid.New().String()


}

func UuidLastPart(id string)string{
	u,err:=uuid.Parse(id)
	if err!=nil{
		u= uuid.New()
	}
	return hex.EncodeToString(u[10:16])
}

func MyHostname() string {
	myHostname, _ := os.Hostname()
	return myHostname
}