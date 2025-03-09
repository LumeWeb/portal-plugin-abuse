package email

import tfidf "github.com/dkgv/go-tf-idf"

// Default weight modifiers for classifier
const (
	// Content weight modifiers
	DefaultWeightQuotedContent   = 0.7 // Weight for level 1 quotes
	DefaultWeightQuoteMultiplier = 0.5 // Additional reduction for each deeper quote level
	DefaultWeightSignature       = 0.3 // Weight for signatures
	DefaultWeightForwarded       = 0.8 // Weight for forwarded content

	// Subject weight multiplier
	DefaultWeightSubjectMultiplier = 2 // Multiplier for terms found in subject

	// Similarity thresholds
	DefaultSimilarityMinimumThreshold  = 0.3  // Minimum similarity for TF-IDF match
	DefaultSimilarityPriorityThreshold = 0.5  // Similarity threshold for boosting priority
	DefaultSimilarityScoreRatio        = 0.55 // Ratio for significant category comparison

	// Score thresholds
	DefaultScoreThreshold          = 5 // Default score threshold for classification
	DefaultScoreHarassment         = 4 // Lower threshold for harassment classification
	DefaultScoreCategoryDifference = 3 // Threshold for category difference comparison
	DefaultScoreReviewThreshold    = 5 // Score threshold for review

	// Priority thresholds
	DefaultPriorityScoreLow    = 2  // Score below this is low priority
	DefaultPriorityScoreMedium = 8  // Score below this is medium priority
	DefaultPriorityScoreHigh   = 15 // Score below this is high priority, above is critical

	// Attachment thresholds
	DefaultAttachmentScoreMedium   = 5  // Attachment score for medium priority
	DefaultAttachmentScoreHigh     = 8  // Attachment score for high priority
	DefaultAttachmentScoreCritical = 12 // Attachment score for critical priority

	// Header analysis thresholds
	DefaultHeaderScoreThreshold       = 5  // Minimum header score to consider relevant
	DefaultHeaderScoreMediumThreshold = 8  // Score that increases priority to medium
	DefaultHeaderScoreHighThreshold   = 12 // Score that increases priority to high
	DefaultHeaderReviewThreshold      = 10 // Score that triggers manual review

	// Negation proximity
	DefaultNegationProximityMax = 5 // Words to look for negation patterns
)

// Default email pattern matches used across email package
var (
	// URL patterns
	DefaultURLPattern = `https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`

	// URL defanging pattern - handles various common techniques for defanging URLs
	DefaultDefangPattern = `(hxxps?://|h\[t\]tps?://|h\.\.ps?://|https?://[^\s]*\[\.\]|[^\s]+\[\.\][^\s]*|\[?\.\]|-?\[dot\]-?|(?:\s+dot\s+)|(?:\(dot\)))`
)

// CID and hash pattern constants - kept separate from configuration options
var (
	CIDPattern = `(cid:[a-zA-Z0-9]{24,})`
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

	// Negation patterns that reduce score when found near terms
	DefaultNegationPatterns = []string{
		"not", "no", "never", "isn't", "aren't", "wasn't", "weren't",
		"doesn't", "don't", "didn't", "won't", "wouldn't", "couldn't",
		"shouldn't", "can't", "cannot", "hasn't", "haven't",
		"without", "except", "excluding", "other than", "rather than",
		"legitimate", "authorized", "approved", "permitted",
	}

	// Common signature phrases
	DefaultSignaturePhrases = []string{
		"thanks",
		"regards",
		"cheers",
		"best",
		"sincerely",
		"sent from",
		"mobile",
		"phone",
		"@",
		"--",
		"__",
		"http://",
		"https://",
	}
)

// ClassifierOptions holds configuration options for the classifier
type ClassifierOptions struct {
	// Content weight modifiers
	WeightQuotedContent   float64
	WeightQuoteMultiplier float64
	WeightSignature       float64
	WeightForwarded       float64

	// Subject weight multiplier
	WeightSubjectMultiplier int

	// TF-IDF processor
	tfIdf *tfidf.TfIdf

	// Similarity thresholds
	SimilarityMinimumThreshold  float64
	SimilarityPriorityThreshold float64
	SimilarityScoreRatio        float64

	// Score thresholds
	ScoreThreshold          int
	ScoreHarassment         int
	ScoreCategoryDifference int
	ScoreReviewThreshold    int

	// Priority thresholds
	PriorityScoreLow    int
	PriorityScoreMedium int
	PriorityScoreHigh   int

	// Attachment thresholds
	AttachmentScoreMedium   int
	AttachmentScoreHigh     int
	AttachmentScoreCritical int

	// Header analysis thresholds
	HeaderScoreThreshold       int
	HeaderScoreMediumThreshold int
	HeaderScoreHighThreshold   int
	HeaderReviewThreshold      int
	PhishingDisplayNameScore   int

	// Negation proximity
	NegationProximityMax int

	// Pattern overrides
	URLPattern    string
	DefangPattern string

	// String arrays
	SignaturePatterns []string
	ForwardedPatterns []string
	ForwardingPhrases []string
	AttachmentDomains []string
	NegationPatterns  []string
	SignaturePhrases  []string
}

// DefaultClassifierOptions returns the default configuration options for the classifier
func DefaultClassifierOptions() *ClassifierOptions {
	return &ClassifierOptions{
		// Content weight modifiers
		WeightQuotedContent:   DefaultWeightQuotedContent,
		WeightQuoteMultiplier: DefaultWeightQuoteMultiplier,
		WeightSignature:       DefaultWeightSignature,
		WeightForwarded:       DefaultWeightForwarded,

		// Subject weight multiplier
		WeightSubjectMultiplier: DefaultWeightSubjectMultiplier,

		// Similarity thresholds
		SimilarityMinimumThreshold:  DefaultSimilarityMinimumThreshold,
		SimilarityPriorityThreshold: DefaultSimilarityPriorityThreshold,
		SimilarityScoreRatio:        DefaultSimilarityScoreRatio,

		// Score thresholds
		ScoreThreshold:          DefaultScoreThreshold,
		ScoreHarassment:         DefaultScoreHarassment,
		ScoreCategoryDifference: DefaultScoreCategoryDifference,
		ScoreReviewThreshold:    DefaultScoreReviewThreshold,

		// Priority thresholds
		PriorityScoreLow:    DefaultPriorityScoreLow,
		PriorityScoreMedium: DefaultPriorityScoreMedium,
		PriorityScoreHigh:   DefaultPriorityScoreHigh,

		// Attachment thresholds
		AttachmentScoreMedium:   DefaultAttachmentScoreMedium,
		AttachmentScoreHigh:     DefaultAttachmentScoreHigh,
		AttachmentScoreCritical: DefaultAttachmentScoreCritical,
		// Header analysis thresholds
		HeaderScoreThreshold:       DefaultHeaderScoreThreshold,
		HeaderScoreMediumThreshold: DefaultHeaderScoreMediumThreshold,
		HeaderScoreHighThreshold:   DefaultHeaderScoreHighThreshold,
		HeaderReviewThreshold:      DefaultHeaderReviewThreshold,
		PhishingDisplayNameScore:   3,

		// Negation proximity
		NegationProximityMax: DefaultNegationProximityMax,

		// Pattern overrides
		URLPattern:    DefaultURLPattern,
		DefangPattern: DefaultDefangPattern,

		// String arrays
		SignaturePatterns: DefaultSignaturePatterns,
		ForwardedPatterns: DefaultForwardedPatterns,
		ForwardingPhrases: DefaultForwardingPhrases,
		AttachmentDomains: DefaultAttachmentDomains,
		NegationPatterns:  DefaultNegationPatterns,
		SignaturePhrases:  DefaultSignaturePhrases,
	}
}

// ContentExtractorOptions holds configuration options for the content extractor
type ContentExtractorOptions struct {
	// URL Pattern
	URLPattern string

	// Defang Pattern
	DefangPattern string

	// String arrays
	SignaturePatterns []string
	ForwardedPatterns []string
	ForwardingPhrases []string
	AttachmentDomains []string
	SignaturePhrases  []string

	// Weight modifiers
	WeightQuotedContent   float64
	WeightQuoteMultiplier float64
	WeightSignature       float64
	WeightForwarded       float64
}

// DefaultContentExtractorOptions returns the default configuration options for the content extractor
func DefaultContentExtractorOptions() *ContentExtractorOptions {
	return &ContentExtractorOptions{
		// URL Pattern
		URLPattern: DefaultURLPattern,

		// Defang Pattern
		DefangPattern: DefaultDefangPattern,

		// String arrays
		SignaturePatterns: DefaultSignaturePatterns,
		ForwardedPatterns: DefaultForwardedPatterns,
		ForwardingPhrases: DefaultForwardingPhrases,
		AttachmentDomains: DefaultAttachmentDomains,
		SignaturePhrases:  DefaultSignaturePhrases,

		// Weight modifiers
		WeightQuotedContent:   DefaultWeightQuotedContent,
		WeightQuoteMultiplier: DefaultWeightQuoteMultiplier,
		WeightSignature:       DefaultWeightSignature,
		WeightForwarded:       DefaultWeightForwarded,
	}
}

// Option is a function that modifies a options struct
type ContentExtractorOption func(*ContentExtractorOptions)

// WithWeightQuotedContent sets the quoted content weight
func WithWeightQuotedContent(weight float64) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.WeightQuotedContent = weight
	}
}

// WithWeightQuoteMultiplier sets the quote level multiplier
func WithWeightQuoteMultiplier(multiplier float64) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.WeightQuoteMultiplier = multiplier
	}
}

// WithWeightSignature sets the signature weight
func WithWeightSignature(weight float64) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.WeightSignature = weight
	}
}

// WithWeightForwarded sets the forwarded content weight
func WithWeightForwarded(weight float64) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.WeightForwarded = weight
	}
}

// WithSubjectMultiplier sets the subject weight multiplier
func WithSubjectMultiplier(multiplier int) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.WeightSubjectMultiplier = multiplier
	}
}

// WithSimilarityThresholds sets all similarity thresholds
func WithSimilarityThresholds(minimum, priority, ratio float64) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.SimilarityMinimumThreshold = minimum
		opts.SimilarityPriorityThreshold = priority
		opts.SimilarityScoreRatio = ratio
	}
}

// WithScoreThresholds sets all score thresholds
func WithScoreThresholds(threshold, harassment, categoryDiff, review int) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.ScoreThreshold = threshold
		opts.ScoreHarassment = harassment
		opts.ScoreCategoryDifference = categoryDiff
		opts.ScoreReviewThreshold = review
	}
}

// WithPriorityThresholds sets all priority thresholds
func WithPriorityThresholds(low, medium, high int) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.PriorityScoreLow = low
		opts.PriorityScoreMedium = medium
		opts.PriorityScoreHigh = high
	}
}

// WithAttachmentThresholds sets all attachment thresholds
func WithAttachmentThresholds(medium, high, critical int) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.AttachmentScoreMedium = medium
		opts.AttachmentScoreHigh = high
		opts.AttachmentScoreCritical = critical
	}
}

// WithHeaderAnalysisThresholds sets all header analysis thresholds
func WithHeaderAnalysisThresholds(threshold, medium, high, review int) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.HeaderScoreThreshold = threshold
		opts.HeaderScoreMediumThreshold = medium
		opts.HeaderScoreHighThreshold = high
		opts.HeaderReviewThreshold = review
	}
}

// WithNegationProximityMax sets the negation proximity max
func WithNegationProximityMax(max int) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.NegationProximityMax = max
	}
}

// WithSignaturePatterns sets custom signature detection patterns
func WithSignaturePatterns(patterns []string) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.SignaturePatterns = patterns
	}
}

// WithForwardedPatterns sets custom forwarded message patterns
func WithForwardedPatterns(patterns []string) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.ForwardedPatterns = patterns
	}
}

// WithNegationPatterns sets custom negation patterns
func WithNegationPatterns(patterns []string) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.NegationPatterns = patterns
	}
}

// WithAttachmentDomains sets custom attachment domains
func WithAttachmentDomains(domains []string) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.AttachmentDomains = domains
	}
}

// WithURLPatterns sets all URL-related patterns
func WithURLPatterns(url, defang string) ClassifierOption {
	return func(opts *ClassifierOptions) {
		opts.URLPattern = url
		opts.DefangPattern = defang
	}
}

// WithContentExtractorWeights sets the content extractor weights
func WithContentExtractorWeights(quoted, multiplier, signature, forwarded float64) ContentExtractorOption {
	return func(opts *ContentExtractorOptions) {
		opts.WeightQuotedContent = quoted
		opts.WeightQuoteMultiplier = multiplier
		opts.WeightSignature = signature
		opts.WeightForwarded = forwarded
	}
}

// WithContentExtractorPatterns sets the content extractor patterns
func WithContentExtractorPatterns(url, defang string) ContentExtractorOption {
	return func(opts *ContentExtractorOptions) {
		opts.URLPattern = url
		opts.DefangPattern = defang
	}
}

// WithContentExtractorSignaturePatterns sets the content extractor signature patterns
func WithContentExtractorSignaturePatterns(patterns []string) ContentExtractorOption {
	return func(opts *ContentExtractorOptions) {
		opts.SignaturePatterns = patterns
	}
}

// WithContentExtractorForwardedPatterns sets the content extractor forwarded patterns
func WithContentExtractorForwardedPatterns(patterns []string, phrases []string) ContentExtractorOption {
	return func(opts *ContentExtractorOptions) {
		opts.ForwardedPatterns = patterns
		opts.ForwardingPhrases = phrases
	}
}

// WithContentExtractorAttachmentDomains sets the content extractor attachment domains
func WithContentExtractorAttachmentDomains(domains []string) ContentExtractorOption {
	return func(opts *ContentExtractorOptions) {
		opts.AttachmentDomains = domains
	}
}

// WithContentExtractorSignaturePhrases sets the content extractor signature phrases
func WithContentExtractorSignaturePhrases(phrases []string) ContentExtractorOption {
	return func(opts *ContentExtractorOptions) {
		opts.SignaturePhrases = phrases
	}
}

// ContentExtractorOptionsFromClassifier creates content extractor options that match a classifier's options
func ContentExtractorOptionsFromClassifier(classifierOpts *ClassifierOptions) []ContentExtractorOption {
	return []ContentExtractorOption{
		WithContentExtractorWeights(
			classifierOpts.WeightQuotedContent,
			classifierOpts.WeightQuoteMultiplier,
			classifierOpts.WeightSignature,
			classifierOpts.WeightForwarded,
		),
		WithContentExtractorPatterns(
			classifierOpts.URLPattern,
			classifierOpts.DefangPattern,
		),
		WithContentExtractorSignaturePatterns(classifierOpts.SignaturePatterns),
		WithContentExtractorForwardedPatterns(
			classifierOpts.ForwardedPatterns,
			classifierOpts.ForwardingPhrases,
		),
		WithContentExtractorAttachmentDomains(classifierOpts.AttachmentDomains),
		WithContentExtractorSignaturePhrases(classifierOpts.SignaturePhrases),
	}
}

const exampleCID = "QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D"
