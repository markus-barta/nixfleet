package colors

import (
	"fmt"
	"regexp"
	"strings"
)

// UpdateHostPalette updates the hostPalette entry in theme-palettes.nix content.
// It finds the hostPalette block and adds/updates the hostname = "palette"; entry.
func UpdateHostPalette(content, hostname, paletteName string) (string, error) {
	// Find hostPalette block
	hostPaletteRegex := regexp.MustCompile(`(?s)(hostPalette\s*=\s*\{)(.*?)(\};)`)
	matches := hostPaletteRegex.FindStringSubmatch(content)
	if len(matches) != 4 {
		return "", fmt.Errorf("could not find hostPalette block in theme-palettes.nix")
	}

	hostPaletteStart := matches[1]
	hostPaletteBody := matches[2]
	hostPaletteEnd := matches[3]

	// Check if hostname already exists
	escapedHostname := regexp.QuoteMeta(hostname)
	entryRegex := regexp.MustCompile(fmt.Sprintf(`(?m)^(\s*)(%s|"%s")\s*=\s*"[^"]+";`, escapedHostname, escapedHostname))

	// Determine if hostname needs quoting (contains hyphen)
	quotedHostname := hostname
	if strings.Contains(hostname, "-") {
		quotedHostname = fmt.Sprintf(`"%s"`, hostname)
	}

	newEntry := fmt.Sprintf(`%s = "%s";`, quotedHostname, paletteName)

	if entryRegex.MatchString(hostPaletteBody) {
		// Replace existing entry
		hostPaletteBody = entryRegex.ReplaceAllStringFunc(hostPaletteBody, func(match string) string {
			// Preserve indentation
			indent := ""
			if idx := strings.Index(match, hostname); idx > 0 {
				indent = match[:idx]
			} else if idx := strings.Index(match, `"`); idx > 0 {
				indent = match[:idx]
			}
			return indent + newEntry
		})
	} else {
		// Add new entry (before the closing brace, maintaining indentation)
		lines := strings.Split(hostPaletteBody, "\n")
		indent := "    " // Default indentation
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				// Extract indentation from existing entry
				idx := strings.Index(line, trimmed)
				if idx > 0 {
					indent = line[:idx]
					break
				}
			}
		}

		// Find a good place to insert (after the last non-comment line)
		insertIdx := len(lines) - 1
		for i := len(lines) - 1; i >= 0; i-- {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				insertIdx = i + 1
				break
			}
		}

		// Insert the new entry
		newLine := indent + newEntry
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:insertIdx]...)
		newLines = append(newLines, newLine)
		newLines = append(newLines, lines[insertIdx:]...)
		hostPaletteBody = strings.Join(newLines, "\n")
	}

	// Reconstruct the full content
	newContent := hostPaletteRegex.ReplaceAllString(content, hostPaletteStart+hostPaletteBody+hostPaletteEnd)
	return newContent, nil
}

// GenerateCustomPalette generates Nix code for a custom palette entry.
// This is inserted into the palettes = { ... } block.
func GenerateCustomPalette(hostname string, palette *Palette) string {
	name := fmt.Sprintf("custom-%s", hostname)

	return fmt.Sprintf(`
    # P2950: Auto-generated custom palette for %s
    %s = {
      name = "%s";
      category = "custom";
      description = "%s";

      gradient = {
        lightest = "%s";
        primary = "%s";
        secondary = "%s";
        midDark = "%s";
        dark = "%s";
        darker = "%s";
        darkest = "%s";
      };

      text = {
        onLightest = "%s";
        onMedium = "%s";
        accent = "%s";
        muted = "%s";
        mutedLight = "%s";
      };

      zellij = {
        bg = "%s";
        fg = "%s";
        frame = "%s";
        black = "%s";
        white = "%s";
        highlight = "%s";
      };
    };
`,
		hostname,
		name,
		palette.Name,
		palette.Description,
		palette.Gradient.Lightest,
		palette.Gradient.Primary,
		palette.Gradient.Secondary,
		palette.Gradient.MidDark,
		palette.Gradient.Dark,
		palette.Gradient.Darker,
		palette.Gradient.Darkest,
		palette.Text.OnLightest,
		palette.Text.OnMedium,
		palette.Text.Accent,
		palette.Text.Muted,
		palette.Text.MutedLight,
		palette.Zellij.Bg,
		palette.Zellij.Fg,
		palette.Zellij.Frame,
		palette.Zellij.Black,
		palette.Zellij.White,
		palette.Zellij.Highlight,
	)
}

// InsertCustomPalette adds a new custom palette to the palettes block.
// It inserts before the closing }; of the palettes block.
// P1000: Fixed to find correct closing brace using brace counting.
func InsertCustomPalette(content, paletteNix string) (string, error) {
	// First, check if there's already a custom palettes section
	customMarker := "# Custom palettes (auto-generated)"
	if strings.Contains(content, customMarker) {
		// Insert after the marker
		idx := strings.Index(content, customMarker)
		endOfMarker := idx + len(customMarker)
		// Find the next newline
		nextNewline := strings.Index(content[endOfMarker:], "\n")
		if nextNewline == -1 {
			return "", fmt.Errorf("malformed theme-palettes.nix: no newline after custom palettes marker")
		}
		insertPoint := endOfMarker + nextNewline + 1
		return content[:insertPoint] + paletteNix + content[insertPoint:], nil
	}

	// No marker exists - find the end of the palettes block by brace counting
	// P1000: Must find "palettes = {" and count braces to find its matching close
	palettesStart := strings.Index(content, "palettes = {")
	if palettesStart == -1 {
		// Try alternate format
		palettesStart = strings.Index(content, "palettes= {")
		if palettesStart == -1 {
			palettesStart = strings.Index(content, "palettes ={")
			if palettesStart == -1 {
				return "", fmt.Errorf("could not find palettes block in theme-palettes.nix")
			}
		}
	}

	// Find the opening brace
	openBraceIdx := strings.Index(content[palettesStart:], "{")
	if openBraceIdx == -1 {
		return "", fmt.Errorf("could not find opening brace for palettes block")
	}
	openBraceIdx += palettesStart

	// Count braces to find the matching close
	braceCount := 0
	palettesEndIdx := -1
	for i := openBraceIdx; i < len(content); i++ {
		if content[i] == '{' {
			braceCount++
		} else if content[i] == '}' {
			braceCount--
			if braceCount == 0 {
				palettesEndIdx = i
				break
			}
		}
	}

	if palettesEndIdx == -1 {
		return "", fmt.Errorf("could not find end of palettes block (unbalanced braces)")
	}

	// Insert the custom palette section before the closing }
	// The closing is at palettesEndIdx, followed by ; at palettesEndIdx+1
	insertion := "\n    # Custom palettes (auto-generated)\n" + paletteNix
	return content[:palettesEndIdx] + insertion + content[palettesEndIdx:], nil
}

// UpdateOrInsertCustomPalette handles updating an existing custom palette or inserting a new one.
func UpdateOrInsertCustomPalette(content, hostname string, palette *Palette) (string, error) {
	paletteName := fmt.Sprintf("custom-%s", hostname)
	paletteNix := GenerateCustomPalette(hostname, palette)

	// Check if this custom palette already exists by looking for the marker comment
	markerComment := fmt.Sprintf("# P2950: Auto-generated custom palette for %s", hostname)
	if strings.Contains(content, markerComment) {
		// Find the start of the custom palette block
		markerIdx := strings.Index(content, markerComment)
		
		// Find the palette name line after the marker
		afterMarker := content[markerIdx:]
		paletteStartIdx := strings.Index(afterMarker, paletteName+" = {")
		if paletteStartIdx == -1 {
			// Marker exists but palette block is malformed, just insert new
			return InsertCustomPalette(content, paletteNix)
		}
		
		// Find the closing }; for this palette by counting braces
		blockStart := markerIdx + paletteStartIdx + len(paletteName+" = {")
		braceCount := 1
		blockEnd := blockStart
		for i := blockStart; i < len(content) && braceCount > 0; i++ {
			if content[i] == '{' {
				braceCount++
			} else if content[i] == '}' {
				braceCount--
			}
			blockEnd = i
		}
		
		// Include the closing semicolon and newline
		if blockEnd+1 < len(content) && content[blockEnd+1] == ';' {
			blockEnd += 2 // Include };
		}
		
		// Replace the entire block (from marker to end of palette)
		return content[:markerIdx] + paletteNix + content[blockEnd:], nil
	}

	// Insert new
	return InsertCustomPalette(content, paletteNix)
}

