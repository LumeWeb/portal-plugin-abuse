package validation

import (
	"regexp"

	z "github.com/Oudwins/zog"
)

// Regular expression for valid confirmation numbers
var confirmationNumberRegex = regexp.MustCompile(`^AR-[a-zA-Z0-9]{8}$`)

// ConfirmationNumberSchema validates confirmation numbers
var ConfirmationNumberSchema = z.String().TestFunc(func(v any, ctx z.Ctx) bool {
	s, ok := v.(*string)
	if !ok {
		return false
	}
	return confirmationNumberRegex.MatchString(*s)
}, z.Message("Invalid confirmation number format"))

// EmailSchema validates email addresses with proper formatting
var EmailSchema = z.String().Email()

// IsValidConfirmationNumber checks if a confirmation number has a valid format
func IsValidConfirmationNumber(confirmationNumber string) bool {
	return confirmationNumberRegex.MatchString(confirmationNumber)
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
