//go:build ignore

package main

import (
	"encoding/json"
	"go/format"
	"html/template"
	"os"
	"path"
	"regexp"
	"strings"
	"unicode"
)

// a regexp to identify word separators which are not underscores
var wordSepRe *regexp.Regexp

func init() {
	wordSepRe = regexp.MustCompile(`[-/ \[\]]+`)
}

// Item struct is either a dataref or command item with a name attribute.
type Item struct {
	Name string `json:"name"`
}

// ItemData is the way the data comes wrapped from /api/v2/datarefs or /api/v2/commands
type ItemData struct {
	Data []*Item `json:"data"`
}

const namesTemplate string = `//
// This file is generated, and changes made directly to this file will be overwritten.  To update
// this file, modify either {{ .JSONFile }} or gen_names.go and then execute 'go generate'.

// Package {{ .Package }} provides known names as string constants to limit repetition of string
// literals and the risk of typos that can't be caught during lint/compile.
package {{ .Package }}

const ({{ range .Items }}
	{{ .Name | toIdentifier }} string = "{{ .Name }}"{{ end }}
)
`

type genCfg struct {
	items    []*Item
	goFile   string
	jsonFile string
	pkg      string
}

type namesGenerator struct {
	genCfgs []*genCfg
}

func (g *namesGenerator) run() error {
	for _, gen := range g.genCfgs {
		if err := g.loadData(gen); err != nil {
			return err
		}
		if err := g.generateFile(gen); err != nil {
			return err
		}
		if err := g.formatFile(gen); err != nil {
			return err
		}
	}

	return nil
}

// converttoIdentifier preps a command or dataref name as an identifier.  We camelcase the path but
// for the trailing portion we just clean up the whitespace.  We cannot camelcase the end of the
// identifier because dataref names are case sensitive, and camelcase can cause conflicts.
// E.g. for:
//
//	SimFlightmodelPositionQ string = "sim/flightmodel/position/Q"
//	SimFlightmodelPositionQ string = "sim/flightmodel/position/q"
//
// So instead, we aim for:
//
//	SimFlightmodelPosition_Q string = "sim/flightmodel/position/Q"
//	SimFlightmodelPosition_q string = "sim/flightmodel/position/q"
//
// Everything after the final / in the name string will be kept with its original casing, and
// underscores will be used for all whitespace.
func convertToIdentifier(name string) string {
	return strings.Join([]string{
		toCamelCase(path.Dir(name)),
		toCleanName(path.Base(name)),
	}, "_")
}

func toCleanName(s string) string {
	// all word separation must be underscores
	s = wordSepRe.ReplaceAllString(s, "_")
	// we don't need trailing underscores (occurs with values like "blah[5]")
	s = strings.TrimSuffix(s, "_")
	return s
}

// toCamelCase is for converting the path of the name to camelcase.
func toCamelCase(s string) string {
	// Convert slashe, hypnens, and spaces to underscores so we only have one word separator.
	// Also catch numeric indexes on datarefs like something[5].

	wordSeps := regexp.MustCompile(`[-/ \[\]]+`)
	s = wordSeps.ReplaceAllString(s, "_")

	// capitalize words
	runes := []rune(s)
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

func (g *namesGenerator) generateFile(gen *genCfg) error {
	templates := template.New("")
	templates.Funcs(template.FuncMap{
		"toIdentifier": convertToIdentifier,
	})

	templates.Parse(namesTemplate)

	fileHandle, err := os.Create(gen.goFile)
	if err != nil {
		return err
	}
	defer fileHandle.Close()

	context := map[string]any{
		"Package":  gen.pkg,
		"JSONFile": gen.jsonFile,
		"Items":    gen.items,
	}

	return templates.Execute(fileHandle, context)
}

func (g *namesGenerator) formatFile(gen *genCfg) error {
	data, err := os.ReadFile(gen.goFile)
	if err != nil {
		return err
	}

	formattedData, err := format.Source(data)
	if err != nil {
		return err
	}

	fileHandle, err := os.Create(gen.goFile)
	if err != nil {
		return err
	}
	defer fileHandle.Close()

	_, err = fileHandle.Write(formattedData)
	return err
}

func (g *namesGenerator) loadData(gen *genCfg) error {
	data, err := os.ReadFile(gen.jsonFile)
	if err != nil {
		return err
	}
	itemData := &ItemData{}
	if err := json.Unmarshal(data, &itemData); err != nil {
		return err
	}

	gen.items = itemData.Data

	return nil
}

func newNamesGenerator() namesGenerator {
	return namesGenerator{
		genCfgs: []*genCfg{
			{
				goFile:   "names/command/commands_gen.go",
				jsonFile: "data/commands.json",
				pkg:      "command",
			},
			{
				goFile:   "names/dataref/datarefs_gen.go",
				jsonFile: "data/datarefs.json",
				pkg:      "dataref",
			},
		},
	}
}

func main() {
	generator := newNamesGenerator()
	err := generator.run()
	if err != nil {
		panic(err)
	}
}
