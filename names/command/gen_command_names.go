//go:build ignore

//go:generate go run commandnames.go

package main

import (
	"encoding/json"
	"go/format"
	"html/template"
	"os"
	"regexp"
	"strings"
	"unicode"
)

type Command struct {
	Name string `json:"name"`
}

type CommandData struct {
	Data []*Command `json:"data"`
}

const commandNamesTemplate string = `//
// This file is generated, and changes made directly to this file will be overwritten.  To update
// this file, modify either commands.json or commandnames.go and then execute 'go generate'.

package command

const ({{ range .Commands }}
	{{ .Name | toIdentifier }} string = "{{ .Name }}"{{ end }}
)
`

type commandNamesGenerator struct {
	commands []*Command
	fileName string
}

func (g *commandNamesGenerator) run() error {
	if err := g.loadCommands(); err != nil {
		return err
	}
	if err := g.generateFile(); err != nil {
		return err
	}
	return g.formatFile()
}

func convertToIdentifier(name string) string {
	// convert slashes and hyphens to underscores so we only have one word separator
	wordSeps := regexp.MustCompile("[-/]+")
	name = wordSeps.ReplaceAllString(name, "_")

	// capitalize words
	runes := []rune(name)
	for idx := range runes {
		if idx == 0 {
			// uppercase first character
			runes[idx] = unicode.ToUpper(runes[idx])
		} else if runes[idx-1] == '_' {
			// uppercase characters after a slash
			runes[idx] = unicode.ToUpper(runes[idx])
		}
	}

	// drop word separators
	return strings.ReplaceAll(string(runes), "_", "")
}

func (g *commandNamesGenerator) generateFile() error {
	templates := template.New("")
	templates.Funcs(template.FuncMap{
		"toIdentifier": convertToIdentifier,
	})

	templates.Parse(commandNamesTemplate)

	fileHandle, err := os.Create(g.fileName)
	if err != nil {
		return err
	}
	defer fileHandle.Close()

	context := map[string]any{
		"Commands": g.commands,
	}

	return templates.Execute(fileHandle, context)
}

func (g *commandNamesGenerator) formatFile() error {
	data, err := os.ReadFile(g.fileName)
	if err != nil {
		return err
	}

	formattedData, err := format.Source(data)
	if err != nil {
		return err
	}

	fileHandle, err := os.Create(g.fileName)
	if err != nil {
		return err
	}
	defer fileHandle.Close()

	_, err = fileHandle.Write(formattedData)
	return err
}

func (g *commandNamesGenerator) loadCommands() error {
	data, err := os.ReadFile("commands.json")
	if err != nil {
		return err
	}
	commandData := &CommandData{}
	if err := json.Unmarshal(data, &commandData); err != nil {
		return err
	}
	g.commands = commandData.Data
	return nil
}

func newCommandNamesGenerator() commandNamesGenerator {
	return commandNamesGenerator{
		fileName: "command_names.go",
	}
}

func main() {
	generator := newCommandNamesGenerator()
	err := generator.run()
	if err != nil {
		panic(err)
	}
}
