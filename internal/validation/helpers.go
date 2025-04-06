package validation

import (
	"regexp"

	z "github.com/Oudwins/zog"
)

// Regular expression for valid case references (12-char-uuid)
var caseReferenceRegex = regexp.MustCompile(`^[A-Z0-9]{12}$`)

// CaseReferenceSchema validates case references
var CaseReferenceSchema = z.String().TestFunc(func(v any, ctx z.Ctx) bool {
	s, ok := v.(*string)
	if !ok {
		return false
	}
	return caseReferenceRegex.MatchString(*s)
}, z.Message("Invalid case reference format"))

// EmailSchema validates email addresses with proper formatting
var EmailSchema = z.String().Email()

// IsValidCaseReference checks if a case reference has a valid format
func IsValidCaseReference(ref string) bool {
	return caseReferenceRegex.MatchString(ref)
}

// FormatZogErrors converts zog validation errors to a user-friendly map
func FormatZogErrors(errs z.ZogIssueMap) map[string]string {
	if errs == nil {
		return nil
	}

	sanitized := z.Issues.SanitizeMap(errs)
	result := make(map[string]string)

	for path, messages := range sanitized {
		if len(messages) > 0 {
			result[path] = messages[0]
		}
	}

	return result
}
