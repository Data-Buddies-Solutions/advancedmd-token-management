package workspace

import (
	"embed"
	"strings"
)

//go:embed files/*.md
var files embed.FS

// Variables returns the workspace markdown files as a map of variable name to content.
// Keys match what ElevenLabs expects as dynamic variables.
func Variables() (map[string]string, error) {
	mapping := map[string]string{
		"identity":     "files/IDENTITY.md",
		"soul":         "files/SOUL.md",
		"user_context": "files/USER.md",
		"tools":        "files/TOOLS.md",
		"voice":        "files/VOICE.md",
	}

	vars := make(map[string]string, len(mapping))
	for key, path := range mapping {
		data, err := files.ReadFile(path)
		if err != nil {
			return nil, err
		}
		vars[key] = strings.TrimSpace(string(data))
	}
	return vars, nil
}
