package tpl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Masterminds/sprig"
	"gopkg.in/yaml.v2"
	"io"
	"os"
	"path"
	"strings"
	"text/template"
)

func executeTemplates(values map[string]interface{}, out io.Writer, tplFile string) error {
	tpl, err := template.New(path.Base(tplFile)).Funcs(sprig.TxtFuncMap()).ParseFiles(tplFile)
	if err != nil {
		return fmt.Errorf("Error parsing template(s): %v", err)
	}

	err = tpl.Execute(out, values)
	if err != nil {
		return fmt.Errorf("Failed to parse standard input: %v", err)
	}
	return nil
}

func parseValues(valuesIn io.Reader, format string) (map[string]interface{}, error) {
	buf := bytes.NewBuffer(nil)
	_, err := io.Copy(buf, valuesIn)
	if err != nil {
		return nil, fmt.Errorf("Failed to read standard input: %v", err)
	}

	var values map[string]interface{}

	switch format {
	case "json":
		err = json.Unmarshal(buf.Bytes(), &values)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse standard input: %v", err)
		}
	case "yaml":
		err = yaml.Unmarshal(buf.Bytes(), &values)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse standard input: %v", err)
		}
	default:
		return nil, fmt.Errorf("Unknown format: %s", format)
	}

	return values, nil
}

// ParseTemplate reads a YAML or JSON document from the v file, uses it as values
// for the t template and writes the executed templates to
// the out stream.
func ParseTemplate(t string, v string) error {
	if _, err := os.Open(t); err != nil {
		return fmt.Errorf("Error, can't open file %s", t)
	}

	valuesFile, err := os.Open(v)
	if err != nil {
		return fmt.Errorf("Error, can't open file %s", t)
	}

	var format string
	if strings.HasSuffix(v, ".json") {
		format = "json"
	} else {
		format = "yaml"
	}

	values, err := parseValues(valuesFile, format)
	if err != nil {
		return err
	}

	err = executeTemplates(values, os.Stdout, t)
	if err != nil {
		return err
	}
	return nil
}