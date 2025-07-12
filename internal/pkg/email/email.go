package email

// Default email pattern matches used across email package
var (
	CIDPattern = `(cid:[a-zA-Z0-9\-_.@]+)`
	// URL patterns
	DefaultURLPattern = `https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`

	// URL defanging pattern - handles various common techniques for defanging URLs
	DefaultDefangPattern = `(hxxps?://|h\[t\]tps?://|h\.\.ps?://|https?://[^\s]*\[\.\]|[^\s]+\[\.\][^\s]*|\[?\.\]|-?\[dot\]-?|(?:\s+dot\s+)|(?:\(dot\)))`

	// Common signature phrases
	DefaultSignaturePhrases = []string{
		"thanks",
		"regards",
		"cheers",
		"best",
		"sincerely",
		"sent from",
	}

	// Email signature patterns
	DefaultSignaturePatterns = []string{
		`(?m)^--+\s*$`,                          // Dashed line
		`(?m)^_+\s*$`,                           // Underscore line
		`(?m)^—+\s*$`,                           // Em dash line
		`(?m)^[-—_]{2,}\s*$`,                    // Any divider line
		`(?m)^[^\r\n]+ wrote:$`,                 // "Someone wrote:"
		`(?m)^[\w\s\.\-]+ \| .+@.+\.[a-z]{2,}$`, // Name | Email format
		`(?m)^(Sent from|Sent via|Get|Enviado desde) .+$`,         // Sent from...
		`(?m)^([A-Za-z\s\.]+)?\s*[A-Za-z0-9\s\.,]+\s*[|•]\s*.+$`,  // Formatted signature with pipe/bullet
		`(?m)^Best regards,\s*$`,                                  // Common sign-off
		`(?m)^(Regards|Sincerely|Cheers|Thanks|Thank you),\s*$`,   // Common sign-offs
		`(?m)^[A-Z][a-z]+(\s[A-Z][a-z]+){1,2}\s*$`,                // Name pattern at end of message
		`(?m)^.+\s+[Pp]hone:\s*[\+\d\s\(\)\-\.]{7,}$`,             // Phone line in sig
		`(?m)^.+\s+[Mm]obile:\s*[\+\d\s\(\)\-\.]{7,}$`,            // Mobile line in sig
		`(?m)^www\..+\.[a-z]{2,}\s*$`,                             // Website in sig
		`(?m)^https?://.+\.[a-z]{2,}\s*$`,                         // URL in sig
		`(?m)^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\s*$`, // Email in sig on its own line
	}

	// Email forwarded message patterns
	DefaultForwardedPatterns = []string{
		`---------- Forwarded message ---------`,
		`-----Original Message-----`,
		`Begin forwarded message:`,
		`------- Forwarded Message -------`,
		`Forwarded: `,
		`----- Forwarded by `,
		`From: .+\nDate: .+\nSubject: .+`,         // Common forwarded headers pattern
		`From: .+\nSent: .+\nTo: .+\nSubject: .+`, // Outlook forwarded message pattern
	}

	// Common phrases that indicate forwarded messages
	DefaultForwardingPhrases = []string{
		"forwarded message",
		"original message",
		"begin forwarded",
		"forwarded from",
		"forwarded by",
	}

	// Common attachment domains that are usually benign
	DefaultAttachmentDomains = []string{
		"imgur.com",
		"ibb.co",
		"postimg.cc",
		"i.stack.imgur.com",
		"drive.google.com",
		"docs.google.com",
	}
)

// ContentExtractorOptions holds configuration options for the content extractor
type ContentExtractorOptions struct {
	// URL Pattern
	URLPattern string

	// Defang Pattern
	DefangPattern string

	SignaturePhrases  []string
	SignaturePatterns []string
	ForwardingPhrases []string
	ForwardedPatterns []string
	AttachmentDomains []string
}

// DefaultContentExtractorOptions returns the default configuration options for the content extractor
func DefaultContentExtractorOptions() *ContentExtractorOptions {
	return &ContentExtractorOptions{
		// URL Pattern
		URLPattern: DefaultURLPattern,

		// Defang Pattern
		DefangPattern: DefaultDefangPattern,

		SignaturePhrases:  DefaultSignaturePhrases,
		SignaturePatterns: DefaultSignaturePatterns,
		ForwardedPatterns: DefaultForwardedPatterns,
		ForwardingPhrases: DefaultForwardingPhrases,
		AttachmentDomains: DefaultAttachmentDomains,
	}
}

// Option is a function that modifies a options struct
type ContentExtractorOption func(*ContentExtractorOptions)

// WithURLPatterns sets all URL-related patterns
func WithURLPatterns(url, defang string) ContentExtractorOption {
	return func(opts *ContentExtractorOptions) {
		opts.URLPattern = url
		opts.DefangPattern = defang
	}
}
