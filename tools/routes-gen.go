package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	genFilePath  = "gen/event-gen/paths.gen.go"
	yamlFilePath = "spec/event-api.yml"
	packageName  = "event"
)

type OpenAPI struct {
	Paths map[string]interface{} `yaml:"paths"`
}

func pathToConst(path string) string {
	result := ""
	for _, c := range path {
		if c == '/' {
			result += "_"
		} else if c == '{' || c == '}' {
			continue
		} else {
			result += string(c)
		}
	}
	if result == "_" { // index
		result = "index"
	}
	result = strings.ToTitle(strings.TrimPrefix(result, "_"))
	result = strings.ReplaceAll(result, ".", "_")
	result = strings.ReplaceAll(result, "*", "")
	return result
}

func main() {
	data, err := os.ReadFile(yamlFilePath)
	if err != nil {
		log.Fatalln(err)
	}

	var api OpenAPI
	err = yaml.Unmarshal(data, &api)
	if err != nil {
		log.Fatalln(err)
	}
	os.Remove(genFilePath)

	out := fmt.Sprintf("package %s\n\nconst (\n", packageName)
	for path := range api.Paths {
		constName := pathToConst(path)
		out += fmt.Sprintf("    %s = \"%s\"\n", constName, path)
	}
	out += ")\n"
	err = os.WriteFile(genFilePath, []byte(out), 0644)
	if err != nil {
		log.Fatalln(err)
	}
}
