package core

import (
	"bytes"
	"sync"
	"text/template"

	"github.com/mgutz/ansi"
)

// DisableColor can be used to make testing reliable
var DisableColor = false

var TemplateFuncsWithColor = map[string]interface{}{
	// Templates with Color formatting. See Documentation: https://github.com/mgutz/ansi#style-format
	"color": ansi.ColorCode,
}

var TemplateFuncsNoColor = map[string]interface{}{
	// Templates without Color formatting. For layout/ testing.
	"color": func(color string) string {
		return ""
	},
}

//RunTemplate returns two formatted strings given a template and
//the data it requires. The first string returned is generated for
//user-facing output and may or may not contain ANSI escape codes
//for colored output. The second string does not contain escape codes
//and can be used by the renderer for layout purposes.
func RunTemplate(tmpl string, data interface{}) (string, string, error) {
	tPair, err := getTemplatePair(tmpl)
	if err != nil {
		return "", "", err
	}
	userBuf := bytes.NewBufferString("")
	err = tPair[0].Execute(userBuf, data)
	if err != nil {
		return "", "", err
	}
	layoutBuf := bytes.NewBufferString("")
	err = tPair[1].Execute(layoutBuf, data)
	if err != nil {
		return userBuf.String(), "", err
	}
	return userBuf.String(), layoutBuf.String(), err
}

var (
	memoizedGetTemplate = map[string][2]*template.Template{}

	memoMutex = &sync.RWMutex{}
)

//getTemplatePair returns a pair of compiled templates where the
//first template is generated for user-facing output and the
//second is generated for use by the renderer. The second
//template does not contain any color escape codes, whereas
//the first template may or may not depending on DisableColor.
func getTemplatePair(tmpl string) ([2]*template.Template, error) {
	memoMutex.RLock()
	if t, ok := memoizedGetTemplate[tmpl]; ok {
		memoMutex.RUnlock()
		return t, nil
	}
	memoMutex.RUnlock()

	templatePair := [2]*template.Template{nil, nil}

	templateNoColor, err := template.New("prompt").Funcs(TemplateFuncsNoColor).Parse(tmpl)
	if err != nil {
		return [2]*template.Template{}, err
	}

	templatePair[1] = templateNoColor

	if DisableColor {
		templatePair[0] = templatePair[1]
	} else {
		templateWithColor, err := template.New("prompt").Funcs(TemplateFuncsWithColor).Parse(tmpl)
		templatePair[0] = templateWithColor
		if err != nil {
			return [2]*template.Template{}, err
		}
	}

	memoMutex.Lock()
	memoizedGetTemplate[tmpl] = templatePair
	memoMutex.Unlock()
	return templatePair, nil
}
