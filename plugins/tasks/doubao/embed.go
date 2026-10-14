package doubaoplugin

import (
	_ "embed"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"sync"
)

// Source is the pinned upstream built-in plugin. It is never loaded from a request.
//
//go:embed plugin.js
var Source string

var Runtime = sync.OnceValues(func() (*jsplugin.Engine, error) {
	return jsplugin.Compile(Source, jsplugin.Options{Key: "doubao", Version: "78bd5b1cbf"})
})
