package parser

import "regexp"

// Timestamp format in VRChat logs: "2024.01.15 23:59:59"
const timestampLayout = "2006.01.02 15:04:05"

// Compiled regex patterns for event detection.
var (
	// Matches: "[Behaviour] OnPlayerJoined DisplayName"
	// Matches: "[Behaviour] OnPlayerJoined DisplayName (usr_xxx)"
	// Captures: (1) display name, (2) user ID (optional)
	playerJoinPattern = regexp.MustCompile(
		`\[Behaviour\] OnPlayerJoined (.+?)(?:\s+\((usr_[a-f0-9-]+)\))?$`,
	)

	// Matches: "[Behaviour] OnPlayerLeft DisplayName"
	// Note: OnPlayerLeftRoom is handled by exclusionPatterns
	// Captures: (1) display name
	playerLeftPattern = regexp.MustCompile(
		`\[Behaviour\] OnPlayerLeft (.+)$`,
	)

	// Matches: "[Behaviour] Entering Room: World Name"
	// Captures: (1) world name
	enteringRoomPattern = regexp.MustCompile(
		`\[Behaviour\] Entering Room: (.+)$`,
	)

	// Matches: "[Behaviour] Joining wrld_xxx:instance_id"
	// Excludes: "Joining or Creating", "Joining friend"
	// Captures: (1) world ID, (2) instance ID
	joiningPattern = regexp.MustCompile(
		`\[Behaviour\] Joining (wrld_[a-f0-9-]+):(.+)$`,
	)
)

// exclusionPatterns are patterns that look like events but should be ignored.
var exclusionPatterns = []string{
	"OnPlayerJoined:",     // Different log format
	"OnPlayerLeftRoom",    // Self leaving
	"Joining or Creating", // Not actual join
	"Joining friend",      // Not actual join
}
