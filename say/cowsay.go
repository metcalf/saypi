package say

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/metcalf/saypi/say/internal/cows"
	"github.com/mitchellh/go-wordwrap"
)

type cow struct {
	template string
	maxWidth int
}

var commentRE = regexp.MustCompile("##.*\n")

func newCow(name string) (*cow, error) {
	if name == "" {
		name = "default"
	}

	tmpl, err := cows.Asset(name + ".cow")
	if err != nil {
		return nil, err
	}

	return &cow{
		template: string(tmpl),
		maxWidth: 40,
	}, nil
}

func listAnimals() []string {
	assets := cows.AssetNames()
	for i, name := range assets {
		assets[i] = strings.TrimSuffix(name, ".cow")
	}
	return assets
}

func (c *cow) Say(text, eyes, tongue string, think bool) (string, error) {
	if eyes == "" {
		eyes = "oo"
	}

	if tongue == "" {
		tongue = "  "
	}

	if utf8.RuneCountInString(eyes) != 2 {
		return "", errors.New("Eye string must be exactly two characters or empty")
	}

	if utf8.RuneCountInString(tongue) != 2 {
		return "", errors.New("Tongue string must be exactly two characters or empty")
	}

	txt := c.balloonText(text, think, c.maxWidth) + "\n" + c.cowText(eyes, tongue, think)
	return txt, nil
}

// Adapted from https://github.com/marmelab/gosay
func (c *cow) cowText(eyes, tongue string, think bool) string {
	var thoughts string
	if think {
		thoughts = "o"
	} else {
		thoughts = `\`
	}

	output := c.template
	output = commentRE.ReplaceAllString(output, "")

	replacements := map[string]string{
		"$the_cow = <<\"EOC\";\n": ``,
		"$the_cow = <<EOC;\n":     ``,
		`\\`:        `\`,
		`\@`:        `@`,
		"$eyes":     eyes,
		"$tongue":   tongue,
		"$thoughts": thoughts,
		"EOC\n":     ``,
	}
	for before, after := range replacements {
		output = strings.Replace(output, before, after, -1)
	}

	return output
}

func (c *cow) balloonText(text string, think bool, maxWidth int) string {
	var first, middle, last, only [2]rune
	if think {
		first = [2]rune{'(', ')'}
		middle = [2]rune{'(', ')'}
		last = [2]rune{'(', ')'}
		only = [2]rune{'(', ')'}
	} else {
		first = [2]rune{'/', '\\'}
		middle = [2]rune{'|', '|'}
		last = [2]rune{'\\', '/'}
		only = [2]rune{'<', '>'}
	}

	text = wordwrap.WrapString(text, uint(maxWidth))

	lines := strings.Split(text, "\n")

	maxWidth = 0
	for _, Line := range lines {
		length := utf8.RuneCountInString(Line)
		if length > maxWidth {
			maxWidth = length
		}
	}

	nbLines := len(lines)
	upper := " "
	lower := " "

	for l := maxWidth + 1; l >= 0; l-- {
		upper += "_"
		lower += "-"
	}

	upper += " "
	lower += " "

	if nbLines > 1 {
		newText := ""
		for index, Line := range lines {
			for spaceCount := maxWidth - utf8.RuneCountInString(Line); spaceCount > 0; spaceCount-- {
				Line += " "
			}
			if index == 0 {
				newText = fmt.Sprintf("%c %s %c\n", first[0], Line, first[1])
			} else if index == nbLines-1 {
				newText += fmt.Sprintf("%c %s %c", last[0], Line, last[1])
			} else {
				newText += fmt.Sprintf("%c %s %c\n", middle[0], Line, middle[1])
			}
		}

		return fmt.Sprintf("%s\n%s\n%s", upper, newText, lower)
	}

	return fmt.Sprintf("%s\n%c %s %c\n%s", upper, only[0], lines[0], only[1], lower)
}
