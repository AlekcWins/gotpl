package helm

// this package copy from https://github.com/helm/helm/blob/a3c903e4c62839d24389a1e0cdf550a8d759f054/pkg/engine/engine.go#L198

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

const warnStartDelim = "ERR_START"
const warnEndDelim = "ERR_END"
const recursionMaxNums = 1000

func warnWrap(warn string) string {
	return warnStartDelim + warn + warnEndDelim
}

// 'include' needs to be defined in the scope of a 'tpl' template as
// well as regular file-loaded templates.
func includeFun(t *template.Template, includedNames map[string]int) func(string, interface{}) (string, error) {
	return func(name string, data interface{}) (string, error) {
		var buf strings.Builder
		if v, ok := includedNames[name]; ok {
			if v > recursionMaxNums {
				return "", errors.Wrapf(fmt.Errorf("unable to execute template"), "rendering template has a nested reference name: %s", name)
			}
			includedNames[name]++
		} else {
			includedNames[name] = 1
		}
		err := t.ExecuteTemplate(&buf, name, data)
		includedNames[name]--
		return buf.String(), err
	}
}

// As does 'tpl', so that nested calls to 'tpl' see the templates
// defined by their enclosing contexts.
func tplFun(parent *template.Template, includedNames map[string]int, strict bool) func(string, interface{}) (string, error) {
	return func(tpl string, vals interface{}) (string, error) {
		t, err := parent.Clone()
		if err != nil {
			return "", errors.Wrapf(err, "cannot clone template")
		}

		// Re-inject the missingkey option, see text/template issue https://github.com/golang/go/issues/43022
		// We have to go by strict from our engine configuration, as the option fields are private in Template.
		// TODO: Remove workaround (and the strict parameter) once we build only with golang versions with a fix.
		if strict {
			t.Option("missingkey=error")
		} else {
			t.Option("missingkey=zero")
		}

		// Re-inject 'include' so that it can close over our clone of t;
		// this lets any 'define's inside tpl be 'include'd.
		t.Funcs(template.FuncMap{
			"include": includeFun(t, includedNames),
			"tpl":     tplFun(t, includedNames, strict),
		})

		// We need a .New template, as template text which is just blanks
		// or comments after parsing out defines just adds new named
		// template definitions without changing the main template.
		// https://pkg.go.dev/text/template#Template.Parse
		// Use the parent's name for lack of a better way to identify the tpl
		// text string. (Maybe we could use a hash appended to the name?)
		t, err = t.New(parent.Name()).Parse(tpl)
		if err != nil {
			return "", errors.Wrapf(err, "cannot parse template %q", tpl)
		}

		var buf strings.Builder
		if err := t.Execute(&buf, vals); err != nil {
			return "", errors.Wrapf(err, "error during tpl function execution for %q", tpl)
		}

		// See comment in renderWithReferences explaining the <no value> hack.
		return strings.ReplaceAll(buf.String(), "<no value>", ""), nil
	}
}

// InitFunMap creates the Engine's FuncMap and adds context-specific functions.
func InitFunMap(t *template.Template) template.FuncMap {
	includedNames := make(map[string]int)

	// Add the template-rendering functions here so we can close over t.
	funcMap := funcMap()
	funcMap["include"] = includeFun(t, includedNames)
	// TODO strict
	strict := true
	funcMap["tpl"] = tplFun(t, includedNames, strict)

	// Add the `required` function here so we can use lintMode
	funcMap["required"] = func(warn string, val interface{}) (interface{}, error) {
		if val == nil {
			return val, errors.Errorf(warnWrap(warn))
		} else if _, ok := val.(string); ok {
			if val == "" {
				return val, errors.Errorf(warnWrap(warn))
			}
		}
		return val, nil
	}

	// Override sprig fail function for linting and wrapping message
	funcMap["fail"] = func(msg string) (string, error) {
		return "", errors.New(warnWrap(msg))
	}

	return funcMap
}
