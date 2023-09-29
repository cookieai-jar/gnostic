package generator

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

type Description struct {
	Summary string
	Text    string
}

func parseDescription(text string) (*Description, error) {
	var summary, description string
	comments := readDescription(text)
	if len(comments) > 0 {
		summary = strings.TrimSpace(comments[0])
		description = strings.TrimSpace(strings.Join(comments[1:], "\n\n"))
	} else {
		description = ""
	}

	importRoot, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "error determing working directory")
	}

	testTemplate, err := template.New("description").Funcs(template.FuncMap{
		"import": func(path string) string {
			text, err := readFile(importRoot, path)
			if err != nil {
				log.Fatalf("Error reading file: %s", err)
			}
			return text
		},
	}).Parse(description)
	if err != nil {
		return nil, errors.Wrap(err, "error building template")
	}

	var s strings.Builder
	err = testTemplate.Execute(&s, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error executing template")
	}

	return &Description{
		Text:    s.String(),
		Summary: summary,
	}, nil
}

// readDescription reads comments, removing any buf lines
func readDescription(description string) []string {
	lines := strings.Split(description, "\n")
	if len(lines) == 0 {
		return []string{}
	}

	comments := make([]string, 0, len(lines))
	for _, line := range lines {
		// ignore buf directives
		if strings.Contains(line, "buf:") {
			continue
		}
		comments = append(comments, line)
	}
	return comments
}

func readFile(importRoot, path string) (string, error) {
	fullPath := fmt.Sprintf("%s/%s", importRoot, path)
	importedFile, err := os.Open(fullPath)
	if err != nil {
		return "", errors.Wrapf(err, "error opening file: %s", fullPath)
	}
	defer func() { _ = importedFile.Close() }()

	byteValue, err := io.ReadAll(importedFile)
	if err != nil {
		return "", errors.Wrapf(err, "error reading file: %s", fullPath)
	}
	return string(byteValue), nil
}
