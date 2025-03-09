package urlparser // Assuming this is in your abuse plugin package

import (
	"go.lumeweb.com/portal/core"                     // Import core to check registered protocols
	coreTesting "go.lumeweb.com/portal/core/testing" // Your test context setup
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// isProtocolRegistered checks if a protocol with the given name is present
// in the core protocol registry.
// NOTE: Ensure core.GetProtocols() is accessible and returns the current registry.
func isProtocolRegistered(name string) bool {
	protocolsMap := core.GetProtocols() // Adjust path/method if needed
	_, exists := protocolsMap[name]
	return exists
}

func TestExtractMultihashesFromURL(t *testing.T) {
	// Mock logger (or use real one depending on context setup)
	ctx := coreTesting.NewTestContext(t)
	logger := ctx.Logger()

	// It's assumed that during test initialization
	// the necessary protocols (like S5, IPFS) might be registered if running integration tests,
	// AND the core fallback parsers are implicitly available via getParsersFromRegistry().

	type testCase struct {
		name             string
		url              string
		expected         []string
		expectError      bool
		requiredProtocol string // Name of the specific protocol plugin needed for this hash format, or "" if core fallback is sufficient.
	}

	testCases := []testCase{
		// --- General & Error Cases ---
		{
			name:             "Invalid URL",
			url:              " ://invalid-url",
			expected:         []string{},
			expectError:      true,
			requiredProtocol: "", // No specific protocol needed to detect URL error
		},
		{
			name:             "Empty URL",
			url:              "",
			expected:         []string{},
			expectError:      false,
			requiredProtocol: "", // No parsing needed
		},
		{
			name:             "No multihashes",
			url:              "https://example.com/some/other/path",
			expected:         []string{},
			expectError:      false,
			requiredProtocol: "", // No parsing needed
		},
		{
			name:             "CIDv0 with extra characters", // Should not be parsed as MH
			url:              "https://example.com/prefixQmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQsuffix",
			expected:         []string{},
			expectError:      false,
			requiredProtocol: "", // Relies on parser boundaries
		},
		{
			name:             "sha3-512 with one missed char", // Invalid Base58 Multihash
			url:              "https://example.com/ipfs/8tX6sAETd3BwBi7gixdX2jLXsPA3Gcci35Xv8Amzv4EMiAMY9njK4SHSMPtafa7wC3ZdEq4HrtAcZkhmHPtqmCA6s",
			expected:         []string{}, // Not meant to match and should return empty slice
			expectError:      false,      // The extraction finds candidates, but parsing fails quietly for fallbacks
			requiredProtocol: "",         // Core Base58 fallback should reject this
		},

		// --- Core Fallback Formats (Generic Multihash) ---
		{
			name:             "sha3-512 Base58",
			url:              "https://example.com/ipfs/8tX6sAETd3BwBi7gixdX2jLXsPA3Gcci35Xv8Amzv4EMiAMY9njK4SHSMPtafa7wC3ZdEq4HrtAcZkhmHPtqmCA6sG",
			expected:         []string{"8tX6sAETd3BwBi7gixdX2jLXsPA3Gcci35Xv8Amzv4EMiAMY9njK4SHSMPtafa7wC3ZdEq4HrtAcZkhmHPtqmCA6sG"},
			expectError:      false,
			requiredProtocol: "", // Should be handled by CoreBase58Parser fallback
		},
		{
			name:             "blake3 Base58",
			url:              "https://example.com/ipfs/gW16Zm9xPNdvTY3EAqxrN4XA6fD6jDSJr3DUVEsioJyVJD",
			expected:         []string{"gW16Zm9xPNdvTY3EAqxrN4XA6fD6jDSJr3DUVEsioJyVJD"},
			expectError:      false,
			requiredProtocol: "", // Should be handled by CoreBase58Parser fallback
		},
		{
			name:             "sha1 Base58",
			url:              "https://example.com/ipfs/5dt8dsCVW7soePahNSE8WsYthG34yA",
			expected:         []string{"5dt8dsCVW7soePahNSE8WsYthG34yA"},
			expectError:      false,
			requiredProtocol: "", // Should be handled by CoreBase58Parser fallback
		},
		{
			name:             "sha2-512 Base58",
			url:              "https://example.com/ipfs/8VwvaT6zyhuGDSxaYizLSw7egTZEZW9yQfaN5PKEtxLCsGtufYLLHcnswpGTAb4SMRao5aD1qgDsYPvrbAYr98TaZN",
			expected:         []string{"8VwvaT6zyhuGDSxaYizLSw7egTZEZW9yQfaN5PKEtxLCsGtufYLLHcnswpGTAb4SMRao5aD1qgDsYPvrbAYr98TaZN"},
			expectError:      false,
			requiredProtocol: "", // Should be handled by CoreBase58Parser fallback
		},
		{
			name:             "sha3-256 Base58",
			url:              "https://example.com/ipfs/W1fWUnq3m9ecS88R49RzHkBU7abh3t778drs8hs4Dwzywd",
			expected:         []string{"W1fWUnq3m9ecS88R49RzHkBU7abh3t778drs8hs4Dwzywd"},
			expectError:      false,
			requiredProtocol: "", // Should be handled by CoreBase58Parser fallback
		},
		// Add tests for CoreHexParser and CoreBase64Parser if needed

		// --- IPFS Protocol Formats ---
		{
			name:        "sha2-256 CIDv0",
			url:         "https://example.com/ipfs/QmNmLqHftaGB1GPyn4ftYjsfuCXw6BKomJTYFFt9nuYJW8",
			expected:    []string{"QmNmLqHftaGB1GPyn4ftYjsfuCXw6BKomJTYFFt9nuYJW8"},
			expectError: false,
		},
		{
			name:        "Multiple CIDv0s in query parameters",
			url:         "https://example.com/?cid1=QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ&cid2=QmR644jMZ7RkKtQ1R23z5EUx4j6sN3tHdS3v7qQ5y3Lw1B",
			expected:    []string{"QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ", "QmR644jMZ7RkKtQ1R23z5EUx4j6sN3tHdS3v7qQ5y3Lw1B"},
			expectError: false,
		},
		{
			name:        "CIDv0 in path",
			url:         "https://example.com/ipfs/QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ",
			expected:    []string{"QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ"},
			expectError: false,
		},
		{
			name:        "CIDv0 in query parameter",
			url:         "https://example.com/?cid=QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ&other=value",
			expected:    []string{"QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ"},
			expectError: false,
		},
		{
			name:        "CIDv0 in fragment",
			url:         "https://example.com/#QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ",
			expected:    []string{"QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ"},
			expectError: false,
		},
		{
			name:        "CIDv0 in path, query, and fragment (deduplication)",
			url:         "https://example.com/ipfs/QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ?cid=QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ#QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ",
			expected:    []string{"QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ"},
			expectError: false,
		},
		{
			name:             "CIDv1 Base32 in path",
			url:              "https://example.com/ipfs/bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqgb3oabka3jjhkig44y",
			expected:         []string{"bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqgb3oabka3jjhkig44y"},
			expectError:      false,
			requiredProtocol: "ipfs",
		},
		{
			name:             "CIDv1 Base58 in path",
			url:              "https://example.com/ipfs/zQmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ",
			expected:         []string{"zQmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ"},
			expectError:      false,
			requiredProtocol: "ipfs",
		},
		{
			name:        "URL Encoded CIDv0 in path",
			url:         "https://example.com/%2Fipfs%2FQmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ",
			expected:    []string{"QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ"}, // Assumes extractor handles URL decoding before parsing
			expectError: false,
		},
		{
			name:        "CIDv0 in subdomain",
			url:         "https://QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ.example.com/",
			expected:    []string{"QmVLw6VpTuE7jEsX2yW6uLQeT1N5J2LhH9qKQX1AmZdrEQ"}, // Assumes extractor checks subdomains
			expectError: false,
		},
		{
			name:             "CIDv1 in subdomain",
			url:              "https://bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqgb3oabka3jjhkig44y.example.com/",
			expected:         []string{"bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqgb3oabka3jjhkig44y"}, // Assumes extractor checks subdomains
			expectError:      false,
			requiredProtocol: "ipfs",
		},

		// --- S5 Protocol Formats ---
		{
			name:             "S5 Blob ID (hex - small file with BLAKE3)",
			url:              "https://example.com/5b821ee6c198b7c4e185ec87493058c2f688b0f4010df8b0524551364d9cfcd82710eb2a",
			expected:         []string{"5b821ee6c198b7c4e185ec87493058c2f688b0f4010df8b0524551364d9cfcd82710eb2a"},
			expectError:      false,
			requiredProtocol: "s5", // Needs S5 Hex parsing
		},
		{
			name:             "S5 Blob ID (hex - medium file with BLAKE3)",
			url:              "https://example.com/5b821ee6c198b7c4e185ec87493058c2f688b0f4010df8b0524551364d9cfcd82710eb0080",
			expected:         []string{"5b821ee6c198b7c4e185ec87493058c2f688b0f4010df8b0524551364d9cfcd82710eb0080"},
			expectError:      false,
			requiredProtocol: "s5", // Needs S5 Hex parsing
		},
		{
			name:             "S5 Blob ID (hex - small file with SHA256)",
			url:              "https://example.com/5b8212655ed4e809cf124088aae447a747cb9c763aa12e5f9d1ae06e3a3efcbc3d26df64",
			expected:         []string{"5b8212655ed4e809cf124088aae447a747cb9c763aa12e5f9d1ae06e3a3efcbc3d26df64"},
			expectError:      false,
			requiredProtocol: "s5", // Needs S5 Hex parsing
		},
		{
			name:             "S5 Blob ID (hex - large file with BLAKE3)",
			url:              "https://example.com/5b821ee6c198b7c4e185ec87493058c2f688b0f4010df8b0524551364d9cfcd82710eb00000008",
			expected:         []string{"5b821ee6c198b7c4e185ec87493058c2f688b0f4010df8b0524551364d9cfcd82710eb00000008"},
			expectError:      false,
			requiredProtocol: "s5", // Needs S5 Hex parsing
		},
		{
			name:             "S5 Blob ID (base64url - small file)",
			url:              "https://example.com/W4Ie5sGYt8ThheyHSTBYwvaIsPQBDfiwUkVRNk2c_NgnEOsq",
			expected:         []string{"W4Ie5sGYt8ThheyHSTBYwvaIsPQBDfiwUkVRNk2c_NgnEOsq"},
			expectError:      false,
			requiredProtocol: "s5", // Needs S5 Base64URL parsing
		},
		{
			name:             "S5 Blob ID (base32 - medium file)",
			url:              "https://example.com/LOBB5ZWBTC34JYMF5SDUSMCYYL3IRMHUAEG7RMCSIVITMTM47TMCOEHLACAA",
			expected:         []string{"LOBB5ZWBTC34JYMF5SDUSMCYYL3IRMHUAEG7RMCSIVITMTM47TMCOEHLACAA"},
			expectError:      false,
			requiredProtocol: "s5", // Needs S5 Base32 parsing
		},
		{
			name:             "S5 Blob ID (base58 - large file)",
			url:              "https://example.com/22kPJGjG1JL9F6WnTDD5GTyxitAMPesFuWvSzp5rKcAv25JoJoGVFM",
			expected:         []string{"22kPJGjG1JL9F6WnTDD5GTyxitAMPesFuWvSzp5rKcAv25JoJoGVFM"},
			expectError:      false,
			requiredProtocol: "s5", // Needs S5 Base58 parsing
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// --- Conditional Skip Logic ---
			// Check if a specific protocol's parser is needed and if that protocol is registered.
			if tc.requiredProtocol != "" && !isProtocolRegistered(tc.requiredProtocol) {
				t.Skipf("Skipping test, required protocol '%s' is not registered", tc.requiredProtocol)
			}
			// --- End Skip Logic ---

			// Call the core function. This function is expected to use the registry
			// (via ParseStorageHash -> getParsersFromRegistry) which reflects currently
			// loaded protocols + core fallbacks.
			actual, err := ExtractMultihashesFromURL(tc.url, logger) // Replace with actual core function call if different

			if tc.expectError {
				assert.Error(t, err, "Expected an error for URL: %s", tc.url)
			} else {
				assert.NoError(t, err, "Did not expect an error for URL: %s", tc.url)
			}

			// Sort the slices before comparing, as the order of results is not guaranteed.
			sort.Strings(actual)
			sort.Strings(tc.expected)
			assert.Equal(t, tc.expected, actual, "Mismatch in extracted hashes for URL: %s", tc.url)
		})
	}
}
