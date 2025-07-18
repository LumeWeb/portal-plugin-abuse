package email

import (
	"github.com/samber/lo"
)

// ExclusionRule defines patterns that suppress content when matched
type ExclusionRule struct {
	Pattern string // Regex pattern to match
}

// Package-level term dictionaries for classification
var (
	// Terms associated with spam - with severity weights (1-15)
	SpamTerms = map[string]int{
		// Weight 3 - Common but potentially benign terms
		"marketing":     3,
		"newsletter":    3,
		"promotional":   3,
		"advertisement": 3,
		"unsubscribe":   3,
		"opt-out":       3,
		"subscribe":     3,
		"offer":         3,
		"deal":          3,

		// Weight 7 - More specific spam indicators
		"spam":         7,
		"spamming":     7,
		"bulk":         7,
		"unsolicited":  7,
		"unwanted":     7,
		"junk mail":    7,
		"mass email":   7,
		"mass mailing": 7,

		// Weight 10 - Strong spam indicators
		"email abuse":            10,
		"reported as spam":       10,
		"unauthorized marketing": 10,
		"scam":                   10,
		"fraudulent":             10,
		"deceptive marketing":    10,
		"spam campaign":          10,
		"mass spam":              10,
		"spam operation":         10,

		// Weight 12 - Strong spam indicators
		"email harvesting":   12,
		"spambot":            12,
		"guaranteed results": 12,
		"earn money online":  12,
	}

	// Terms associated with harassment - with severity weights (1-15)
	HarassmentTerms = map[string]int{
		// Weight 3 - Potentially concerning but context-dependent
		"inappropriate": 3,
		"offensive":     3,
		"insulting":     3,
		"rude":          3,
		"mean":          3,
		"disrespectful": 3,

		// Weight 7 - More concerning harassment indicators
		"harass":         7,
		"harassment":     7,
		"bully":          7,
		"intimidate":     7,
		"target":         7,
		"victim":         7,
		"hateful":        7,
		"abusive":        7,
		"discriminatory": 7,
		"insult":         7,
		"slur":           7,
		"trolling":       7,

		// Weight 10 - Severe harassment indicators
		"doxing":         10,
		"stalking":       10,
		"cyberstalking":  10,
		"online shaming": 10,
		"outing":         10,

		// Weight 12 - Severe harassment indicators
		"death threat":          12,
		"violent threat":        12,
		"physical threat":       12,
		"sexual harassment":     12,
		"hate speech":           12,
		"racial slur":           12,
		"targeted harassment":   12,
		"persistent harassment": 12,
		"intimidation campaign": 12,
		"mob harassment":        12,
		"coordinated attack":    12,
	}

	// Terms associated with illegal/harmful content - with severity weights (1-15)
	IllegalOrHarmfulTerms = map[string]int{
		// Weight 3 - Basic content concerns
		"terms of service":      3,
		"violation":             3,
		"prohibited":            3,
		"report content":        3,
		"inappropriate content": 3,

		// Weight 7 - More specific content violation indicators
		"dmca":                 7,
		"infringe":             7,
		"unauthorized use":     7,
		"content policy":       7,
		"policy violation":     7,
		"community guidelines": 7,

		// Weight 10 - More specific content violation indicators
		"proprietary":         10,
		"illegal content":     10,
		"promoting violence":  10,
		"glorifying violence": 10,

		// Weight 12 - Severe content violation indicators
		"trademark infringement":  12,
		"formal takedown":         12,
		"legal notice":            12,
		"cease and desist":        12,
		"illegal material":        12,
		"prohibited content":      12,
		"rights violation":        12,
		"content removal request": 12,
		"official dmca":           12,
		"legal action":            12,

		// CSAM is always highest priority
		"csam":                        15,
		"child sexual abuse material": 15,
		"child pornography":           15,
		"cp":                          15,
		"child abuse":                 15,
		"child exploitation":          15,
		"child porn":                  15,
		"minor abuse":                 15,
		"underage content":            15,
		"child sexual abuse":          15,
	}

	// Terms associated with phishing - with severity weights (1-15)
	PhishingTerms = map[string]int{
		// Weight 3 - General security terms
		"verify":     3,
		"account":    3,
		"login":      3,
		"password":   3,
		"security":   3,
		"alert":      3,
		"suspicious": 3,

		// Weight 7 - More specific phishing indicators
		"verification":    7,
		"reset":           7,
		"urgent":          7,
		"immediate":       7,
		"action required": 7,

		// Weight 10 - More specific phishing indicators
		"phishing":       10,
		"credential":     10,
		"authentication": 10,
		"winnings":       10,

		// Weight 12 - Strong phishing signals
		"password reset":       12,
		"account verification": 12,
		"security alert":       12,
		"login attempt":        12,
		"suspicious activity":  12,
		"unauthorized access":  12,
		"banking details":      12,
	}

	// Terms associated with copyright violations - with severity weights (1-15)
	CopyrightTerms = map[string]int{
		// Weight 5 - General copyright terms
		"copyright": 5,
		"license":   5,
		"rights":    5,

		// Weight 7 - General copyright terms
		"infringement": 7,

		// Weight 10 - Specific violation terms
		"takedown":              10,
		"intellectual property": 10,
		"distribution":          10,
		"pirated content":       10,
		"bootleg":               10,

		// Weight 12 - Legal action terms
		"copyright infringement": 12,
		"dmca notice":            12,
		"court order":            12,
		"lawsuit":                12,
		"settlement":             12,

		// Weight 15 - Severe/criminal copyright violations
		"criminal copyright infringement": 15,
		"commercial piracy operation":     15,
		"counterfeit goods":               15,
		"trade secret theft":              15,
		"willful infringement":            15,
		"pre-release piracy":              15,
	}

	// Terms associated with resource abuse - with severity weights (1-15)
	ResourceAbuseTerms = map[string]int{
		// Weight 3 - General resource terms
		"resource":  3,
		"bandwidth": 3,

		// Weight 5 - General resource terms
		"abuse":       5,
		"consumption": 5,
		"utilization": 5,

		// Weight 7 - Specific abuse patterns
		"unauthorized": 7,
		"scraping":     7,

		// Weight 10 - Specific abuse patterns
		"ddos":                10,
		"botnet":              10,
		"mining":              10,
		"credential stuffing": 10,
		"spamming links":      10,

		// Weight 12 - Severe abuse indicators
		"cryptocurrency mining": 12,
		"denial of service":     12,
		"brute force":           12,
		"port scanning":         12,
		"traffic amplification": 12,
	}

	// Terms associated with malware and security threats - with severity weights (1-15)
	MalwareTerms = map[string]int{
		// Weight 5 - Basic security concerns
		"malicious":            5,
		"vulnerability":        5,
		"scan":                 5,
		"antivirus":            5,
		"potentially unwanted": 5,

		// Weight 7 - More specific malware indicators
		"trojan":          7,
		"worm":            7,
		"exploit":         7,
		"spyware":         7,
		"backdoor":        7,
		"infected":        7,
		"security threat": 7,
		"harmful code":    7,
		"malicious code":  7,
		"threat detected": 7,

		// Weight 10 - Severe malware indicators
		"critical vulnerability": 10,
		"zero-day":               10,
		"remote code execution":  10,
		"data breach":            10,
		"malware outbreak":       10,
		"active exploitation":    10,

		// Weight 12 - Severe malware indicators
		"command and control":        12,
		"information stealer":        12,
		"cryptocurrency miner":       12,
		"rootkit":                    12,
		"advanced persistent threat": 12,
	}

	// Urgency patterns with severity weights (1-15)
	UrgencyPatterns = map[string]int{
		// Weight 3 - Basic urgency
		"soon":     3,
		"timely":   3,
		"promptly": 3,
		"quickly":  3,

		// Weight 5 - Higher urgency
		"asap":                5,
		"immediately":         5,
		"expedite":            5,
		"without delay":       5,
		"as soon as possible": 5,
		"time sensitive":      5,

		// Weight 7 - Critical urgency
		"emergency":                 7,
		"critical":                  7,
		"immediate action required": 7,
		"immediate attention":       7,
		"urgent action":             7,
		"time critical":             7,
		"deadline":                  7,
		"urgent matter":             7,
		"escalation":                7,
	}

	// Patterns that may indicate sensitive content with severity weights (1-15)
	SensitivePatterns = map[string]int{
		// Weight 3 - General sensitivity
		"sensitive":    3,
		"private":      3,
		"confidential": 3,
		"personal":     3,

		// Weight 5 - More specific sensitivity concerns
		"minor":                 5,
		"privacy":               5,
		"personal information":  5,
		"financial":             5,
		"identity":              5,
		"personal data":         5,
		"sensitive information": 5,
		"health information":    5,
		"medical information":   5,

		// Weight 7 - Highly sensitive concerns
		"child":            7,
		"underage":         7,
		"illegal":          7,
		"danger":           7,
		"exploitation":     7,
		"vulnerable":       7,
		"child safety":     7,
		"sexual content":   7,
		"explicit content": 7,
		"private images":   7,
		"non-consensual":   7,
		"personal risk":    7,
		"safety risk":      7,
	}

	// Patterns that may indicate threats with severity weights (1-15)
	ThreatPatterns = map[string]int{
		// Weight 3 - Basic security concerns
		"concerning":       3,
		"unusual activity": 3,

		// Weight 5 - More specific threats
		"threat":         5,
		"attack":         5,
		"compromise":     5,
		"hack":           5,
		"breach":         5,
		"security issue": 5,

		// Weight 7 - Severe threats
		"security breach":    7,
		"account hijacking":  7,
		"targeted attack":    7,
		"system compromise":  7,
		"active exploit":     7,
		"widespread attack":  7,
		"security emergency": 7,
		"ongoing breach":     7,
	}

	// Known legitimate no-reply patterns
	NoReplyPatterns = []string{
		"no-reply@", "noreply@", "no.reply@", "donotreply@",
		"do-not-reply@", "do.not.reply@", "automated@", "system@",
		"notification@", "alert@", "newsletter@",
		"news@", "updates@", "admin@", "robot@", "bot@",
	}

	StopWords = []string{
		"a",
		"about",
		"above",
		"after",
		"again",
		"against",
		"all",
		"am",
		"an",
		"and",
		"any",
		"are",
		"aren't",
		"as",
		"at",
		"be",
		"because",
		"been",
		"before",
		"being",
		"below",
		"between",
		"both",
		"but",
		"by",
		"can't",
		"cannot",
		"could",
		"couldn't",
		"did",
		"didn't",
		"do",
		"does",
		"doesn't",
		"doing",
		"don't",
		"down",
		"during",
		"each",
		"few",
		"for",
		"from",
		"further",
		"had",
		"hadn't",
		"has",
		"hasn't",
		"have",
		"haven't",
		"having",
		"he",
		"he'd",
		"he'll",
		"he's",
		"her",
		"here",
		"here's",
		"hers",
		"herself",
		"him",
		"himself",
		"his",
		"how",
		"how's",
		"i",
		"i'd",
		"i'll",
		"i'm",
		"i've",
		"if",
		"in",
		"into",
		"is",
		"isn't",
		"it",
		"it's",
		"its",
		"itself",
		"let's",
		"me",
		"more",
		"most",
		"mustn't",
		"my",
		"myself",
		"no",
		"nor",
		"not",
		"of",
		"off",
		"on",
		"once",
		"only",
		"or",
		"other",
		"ought",
		"our",
		"ours",
		"ourselves",
		"out",
		"over",
		"own",
		"same",
		"shan't",
		"she",
		"she'd",
		"she'll",
		"she's",
		"should",
		"shouldn't",
		"so",
		"some",
		"such",
		"than",
		"that",
		"that's",
		"the",
		"their",
		"theirs",
		"them",
		"themselves",
		"then",
		"there",
		"there's",
		"these",
		"they",
		"they'd",
		"they'll",
		"they're",
		"they've",
		"this",
		"those",
		"through",
		"to",
		"too",
		"under",
		"until",
		"up",
		"very",
		"was",
		"wasn't",
		"we",
		"we'd",
		"we'll",
		"we're",
		"we've",
		"were",
		"weren't",
		"what",
		"what's",
		"when",
		"when's",
		"where",
		"where's",
		"which",
		"while",
		"who",
		"who's",
		"whom",
		"why",
		"why's",
		"with",
		"won't",
		"would",
		"wouldn't",
		"you",
		"you'd",
		"you'll",
		"you're",
		"you've",
		"your",
		"yours",
		"yourself",
		"yourselves",
	}

	StopWordsMap = lo.SliceToMap(StopWords, func(word string) (string, bool) {
		return word, true
	})

	// ExclusionRules defines patterns that prevent false positives
	ExclusionRules = []ExclusionRule{
		{
			Pattern: `(?i)\babuse\s+complaints?\b`,
		},
		{
			Pattern: `(?i)\breports?\s+of\s+abuses?\b`,
		},
		{
			Pattern: `(?i)\b(?:abuse|security)\s+(?:report|complaint)s?\s+received\b`,
		},
		{
			Pattern: `(?i)\b(?:terms of service|privacy policy)\s+updates?\b`,
		},
		{
			Pattern: `(?i)\bfraudulent\s+contents?\b`,
		}, {
			Pattern: `(?i)\babuse\s+department\b`,
		},
	}
)
