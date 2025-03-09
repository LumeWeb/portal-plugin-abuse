package urlparser

import (
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"net/url"
	"regexp"
)

var ( // Content ID pattern
	// CIDv0 pattern (IPFS/IPLD hash patterns)
	CIDv0Pattern = `\b(Qm[1-9A-HJ-NP-Za-km-z]{44})\b`

	// CIDv1 patterns
	CIDv1Base32Pattern = `\b(bafy[a-zA-Z0-9]{52,59})\b`
	CIDv1Base58Pattern = `\b(z[a-zA-Z0-9]{48,59})\b`

	// Other hash patterns
	Base36CIDPattern       = `\b(k[a-zA-Z0-9]{46,59})\b`
	RawBase58Pattern       = `\b([1-9A-HJ-NP-Za-km-z]{30,90})\b` // Updated length
	RawLeadingGPattern     = `\b(g[1-9A-HJ-NP-Za-km-z]{40,59})\b`
	RawLeadingDigitPattern = `\b([0-9][a-zA-Z0-9]{40,59})\b`
	HexHashPattern         = `\b([0-9a-fA-F]{64})\b`

	// S5 Blob ID patterns
	S5HexPattern    = `\b(5b82(1e|12)[0-9a-fA-F]{64,80})\b`
	S5Base64Pattern = `\b(W4I[A-Za-z0-9_-]{43,60})\b` // Removed padding
	S5Base32Pattern = `\b(LOBB[A-Z2-7]{50,90})\b`     // Removed padding
	S5Base58Pattern = `\b(22kP[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]{35,60})\b`
)

var HashRegexes = []*regexp.Regexp{
	regexp.MustCompile(CIDv0Pattern),
	regexp.MustCompile(CIDv1Base32Pattern),
	regexp.MustCompile(CIDv1Base58Pattern),
	regexp.MustCompile(Base36CIDPattern),
	regexp.MustCompile(RawBase58Pattern),
	regexp.MustCompile(RawLeadingGPattern),
	regexp.MustCompile(RawLeadingDigitPattern),
	regexp.MustCompile(HexHashPattern),
	regexp.MustCompile(S5HexPattern),
	regexp.MustCompile(S5Base64Pattern),
	regexp.MustCompile(S5Base32Pattern),
	regexp.MustCompile(S5Base58Pattern),
}

func ExtractMultihashesFromURL(urlStr string, logger *core.Logger) ([]string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return []string{}, err
	}

	potentialLocations := []string{
		parsedURL.Path,
		parsedURL.RawQuery,
		parsedURL.Fragment,
		parsedURL.Host,
	}

	var multihashes []string
	seen := make(map[string]bool)

	for _, location := range potentialLocations {
		if location == "" {
			continue
		}
		decodedLocation, err := url.QueryUnescape(location)
		if err != nil {
			logger.Warn("Warning: Failed to unescape location", zap.String("location", location), zap.Error(err))
			decodedLocation = location
		}

		for _, re := range HashRegexes {
			matches := re.FindAllString(decodedLocation, -1)
			for _, match := range matches {
				_, err := core.ParseStorageHash(match) // Use core.ParseStorageHash
				if err == nil {                        // If ParseStorageHash succeeds, it's a valid hash
					if !seen[match] {
						multihashes = append(multihashes, match)
						seen[match] = true
					}
				} else {
					// Optional: Log the error for debugging purposes.  Don't return the error,
					// as we only want to skip invalid hashes, not abort the entire process.
					logger.Debug("Invalid hash format", zap.String("hash", match), zap.Error(err))
				}
			}
		}
	}

	if len(multihashes) == 0 {
		return []string{}, nil
	}

	return multihashes, nil
}
